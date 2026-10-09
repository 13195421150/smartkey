package handler

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/cardplatform"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

// Bounded locks serialize rotation with preview/redeem, without holding a DB
// transaction while waiting on upstream. The application runs as one process.
var siteCDKLocks [64]sync.Mutex

func lockSiteCDK(code string) func() {
	hash := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	mu := &siteCDKLocks[int(hash[0])%len(siteCDKLocks)]
	mu.Lock()
	return mu.Unlock
}

func lockSiteCDKToken(c *gin.Context, token string) (string, func(), bool) {
	code, err := db.ValidateSiteCDKToken(token)
	if err != nil {
		writeSiteTokenError(c, err)
		return "", nil, false
	}
	unlock := lockSiteCDK(code)
	if _, err = db.ValidateSiteCDKToken(token); err != nil {
		unlock()
		writeSiteTokenError(c, err)
		return "", nil, false
	}
	return code, unlock, true
}
func writeSiteTokenError(c *gin.Context, err error) {
	if errors.Is(err, db.ErrCDKSessionRevoked) {
		c.JSON(400, gin.H{"error": err.Error(), "error_code": "CDK_SESSION_REVOKED"})
		return
	}
	c.JSON(503, gin.H{"error": "暂无法核对兑换会话，请稍后重试"})
}
func terminalCDKFailure(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "failed_precharge", "declined", "cancelled", "canceled", "expired":
		return true
	}
	return false
}
func observeSiteTokenResult(token string, status int, raw []byte) {
	if publicResponseFailed(status, raw) {
		return
	}
	order := extractOrderMap(raw)
	if order != nil && (terminalCDKFailure(strAny(order["status"])) || strAny(order["status"]) == "completed") {
		_ = db.ClearSiteCDKSubmitted(token)
	}
}

// CardPlatformRotateCDK changes only the site's accepted/displayed key.
func CardPlatformRotateCDK(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		RequestID  string `json:"client_request_id"`
		Generation *int64 `json:"generation"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&body) != nil || body.Generation == nil || *body.Generation < 0 || len(body.RequestID) < 16 || len(body.RequestID) > 128 {
		c.JSON(400, gin.H{"error": "换码参数无效，请刷新列表后重试"})
		return
	}
	code, plan, err := db.CanonicalCDKForID(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "本站未找到该完整卡密，请先同步卡密"})
		return
	}
	unlock := lockSiteCDK(code)
	defer unlock()
	current, err := db.SiteCDKFor(code, plan)
	if err != nil {
		c.JSON(503, gin.H{"error": "暂无法读取卡密，请稍后重试"})
		return
	}
	if current.Generation > 0 && current.RequestID == body.RequestID {
		c.JSON(200, gin.H{"id": id, "code": current.Code, "full_code": current.Code, "generation": current.Generation, "site_rotated": true})
		return
	}
	if current.Generation != *body.Generation {
		c.JSON(409, gin.H{"error": db.ErrCDKRotationConflict.Error()})
		return
	}
	cli := cardplatform.NewFromSettings()
	list, err := cli.ListCDKsQuery(c.Request.Context(), cardplatform.CDKListQuery{Page: 1, PageSize: 100, Query: strconv.FormatInt(id, 10)})
	if err != nil {
		writeCardErr(c, err)
		return
	}
	unused := false
	if list != nil {
		for _, it := range list.List {
			if it.ID == id {
				unused = strings.EqualFold(it.Status, "unused")
				break
			}
		}
	}
	if !unused {
		c.JSON(409, gin.H{"error": "只有卡台确认未使用的卡密才可换新；请刷新状态"})
		return
	}
	// An accepted submission may not yet appear in the upstream CDK status.
	tokens, err := db.PendingSiteCDKTokens(code)
	if err != nil {
		c.JSON(503, gin.H{"error": "暂无法核对进行中的兑换任务"})
		return
	}
	for _, token := range tokens {
		st, raw, e := cli.Result(c.Request.Context(), token.Token, token.Device)
		order := extractOrderMap(raw)
		if e != nil || publicResponseFailed(st, raw) || order == nil || !terminalCDKFailure(strAny(order["status"])) {
			c.JSON(409, gin.H{"error": "该卡密仍有尚未确认结束的兑换请求，请先查询进度，勿重复兑换"})
			return
		}
		// This is session validation metadata only; task and failure history stay intact.
		if err = db.ClearSiteCDKSubmittedUpstream(code, token.Token); err != nil {
			c.JSON(503, gin.H{"error": "暂无法核对兑换会话"})
			return
		}
	}
	raw, err := cli.ListCDKOrdersQuery(c.Request.Context(), cardplatform.CDKOrderListQuery{Page: 1, PageSize: 100, CDKID: id})
	if err != nil {
		writeCardErr(c, err)
		return
	}
	var history struct {
		List  []map[string]any `json:"list"`
		Total *int             `json:"total"`
	}
	if json.Unmarshal(raw, &history) != nil || history.List == nil || history.Total == nil || *history.Total < len(history.List) {
		c.JSON(503, gin.H{"error": "卡台兑换记录返回不完整，暂不能换新"})
		return
	}
	for _, order := range history.List {
		if !terminalCDKFailure(strAny(order["status"])) {
			c.JSON(409, gin.H{"error": "该卡密存在进行中或已成功的充值任务，不能换新"})
			return
		}
	}
	next, err := db.RotateSiteCDK(code, plan, body.RequestID, *body.Generation)
	if err != nil {
		if errors.Is(err, db.ErrCDKRotationConflict) {
			c.JSON(409, gin.H{"error": err.Error()})
		} else {
			c.JSON(500, gin.H{"error": "换码保存失败，请用同一次操作重试"})
		}
		return
	}
	auditAdmin(c, "rotate_site_cdk", fmt.Sprintf("upstream_id=%d generation=%d", id, next.Generation))
	c.JSON(http.StatusOK, gin.H{"id": id, "code": next.Code, "full_code": next.Code, "generation": next.Generation, "site_rotated": true})
}

func siteUpstreamToken(c *gin.Context, token string) (string, bool) {
	upstream, err := db.UpstreamCDKToken(token)
	if err != nil {
		writeSiteTokenError(c, err)
		return "", false
	}
	return upstream, true
}

func publicSiteTokenJSON(raw json.RawMessage, upstream, site string) json.RawMessage {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return raw
	}
	var replace func(any) any
	replace = func(v any) any {
		switch item := v.(type) {
		case string:
			if item == upstream {
				return site
			}
		case map[string]any:
			for key, v := range item {
				item[key] = replace(v)
			}
		case []any:
			for i, v := range item {
				item[i] = replace(v)
			}
		}
		return v
	}
	output, err := json.Marshal(replace(value))
	if err != nil {
		return raw
	}
	return output
}
