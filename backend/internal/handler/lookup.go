package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/cardplatform"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

// 公开批量查询上限：每条可能回源卡台 Result，避免一次打爆上游。
const lookupBatchMax = 100
const lookupBatchWorkers = 4

type cdkLookupResult struct {
	OrderRequestID   string                   `json:"-"`
	CDKCode          string                   `json:"cdk_code"`
	Status           string                   `json:"status"` // used only when order_status == completed
	OrderStatus      string                   `json:"order_status,omitempty"`
	OrderID          int64                    `json:"order_id,omitempty"`
	Used             bool                     `json:"used"`
	CanResubmit      bool                     `json:"can_resubmit"`
	AccountEmail     string                   `json:"account_email,omitempty"`
	Plan             string                   `json:"plan,omitempty"`
	UsedAt           *string                  `json:"used_at,omitempty"`
	Notes            string                   `json:"notes,omitempty"`
	Message          string                   `json:"message"`
	FailureReason    string                   `json:"failure_reason,omitempty"`
	ErrorCode        string                   `json:"error_code,omitempty"`
	Stage            string                   `json:"stage,omitempty"`
	CreatedAt        string                   `json:"created_at,omitempty"`
	UpdatedAt        string                   `json:"updated_at,omitempty"`
	Events           []lookupEvent            `json:"events,omitempty"`
	DetailsAvailable bool                     `json:"details_available"`
	LastAttempt      *db.CDKAttemptDiagnostic `json:"last_attempt,omitempty"`
}

// LookupCDKStatus GET /api/v1/lookup/cdk?code=  或兼容 ?cdk_code=
func LookupCDKStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		code = strings.TrimSpace(c.Query("cdk_code"))
	}
	code = strings.TrimSpace(code)
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入卡密"})
		return
	}

	resp := lookupOneCDK(c.Request.Context(), code, deviceFrom(c))
	if resp.Status == "unknown" {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "未找到该卡密记录",
			"cdk_code": code,
			"status":   "unknown",
			"used":     false,
			"message":  "未找到该卡密。请确认输入完整卡密。",
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// LookupCDKStatusBatch POST /api/v1/lookup/cdk/batch
// body: { "codes": ["SXC-…"], "text": "可选整段粘贴" }
func LookupCDKStatusBatch(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req struct {
		Codes []string `json:"codes"`
		Text  string   `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式无效"})
		return
	}
	codes := normalizeLookupCodes(append(append([]string{}, req.Codes...), splitLookupText(req.Text)...))
	if len(codes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入至少一张卡密"})
		return
	}
	if len(codes) > lookupBatchMax {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "一次最多查询 " + strconv.Itoa(lookupBatchMax) + " 张卡密",
			"max":   lookupBatchMax,
		})
		return
	}

	results := make([]cdkLookupResult, len(codes))
	sem := make(chan struct{}, lookupBatchWorkers)
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	device := deviceFrom(c)
	for i, code := range codes {
		wg.Add(1)
		go func(i int, code string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = lookupOneCDK(ctx, code, device)
		}(i, code)
	}
	wg.Wait()

	c.JSON(http.StatusOK, gin.H{
		"total":   len(results),
		"max":     lookupBatchMax,
		"results": results,
	})
}

func splitLookupText(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';' || r == '，' || r == '；'
	})
}

func normalizeLookupCodes(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		code := strings.ToUpper(strings.TrimSpace(item))
		if len(code) < 4 {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func applyLookupFailure(resp *cdkLookupResult, notes string, reusable bool) {
	resp.Status = "failed"
	resp.Used = false
	resp.CanResubmit = reusable
	note := strings.TrimSpace(notes)
	if note != "" {
		resp.Notes = note
	}
	if reusable {
		resp.Message = "使用失败，卡密已重新激活，可以重新提交"
	} else {
		resp.Message = "使用失败，卡密尚未重新激活，请等待处理后再查"
	}
	if note != "" {
		resp.Message = resp.Message + "：" + note
	}
}

// lookupOneCDK never infers payment success from a CDK lifecycle or a saved Session.
func lookupOneCDK(ctx context.Context, code, deviceID string) (resp cdkLookupResult) {
	displayCode := code
	canonical, err := db.ResolveCDKCode(code)
	if err != nil {
		if errors.Is(err, db.ErrCDKAlias) {
			return cdkLookupResult{CDKCode: code, Status: "unknown", Message: "未找到该卡密记录"}
		}
		return cdkLookupResult{CDKCode: code, Status: "unconfirmed", Message: "暂无法核对卡密，请稍后查询"}
	}
	code = canonical
	defer func() {
		attachCDKAttempt(&resp)
		resp.CDKCode = displayCode
	}()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp = cdkLookupResult{CDKCode: code, Status: "unconfirmed", Message: "暂无法确认最新兑换结果，请稍后查询，勿重复提交。"}
	id, plan, _, _, stored := db.LookupStoredCDKByCode(code)
	binding, _ := db.GetBindingByCDK(code)
	if !stored && binding == nil {
		return lookupLegacyCDK(code)
	}
	resp.Plan = plan
	cli := cardplatform.NewFromSettings()
	if id > 0 {
		raw, err := cli.ListCDKOrdersQuery(ctx, cardplatform.CDKOrderListQuery{Page: 1, PageSize: 100, CDKID: id})
		var list struct {
			List []map[string]any `json:"list"`
		}
		if err == nil && json.Unmarshal(raw, &list) == nil && list.List != nil {
			var latest map[string]any
			for _, order := range list.List {
				// Never trust a broad/unfiltered response to authorize a different CDK's order.
				if anyToInt64(order["cdk_id"]) != id {
					continue
				}
				if latest == nil || lookupOrderID(order) > lookupOrderID(latest) {
					latest = order
				}
			}
			if latest != nil {
				// The scoped reconciliation response is independent of browser tokens and webhooks.
				applyLookupOrder(&resp, latest)
				addLookupDetail(ctx, cli, &resp, id)
				return resp
			}
			if len(list.List) == 0 {
				// No order: verify availability with the owner API, not a stale local cache.
				codes, cerr := cli.ListCDKsQuery(ctx, cardplatform.CDKListQuery{Page: 1, PageSize: 100, Query: strconv.FormatInt(id, 10)})
				if cerr == nil && codes != nil {
					for _, item := range codes.List {
						if item.ID == id {
							applyLookupLifecycle(&resp, item.Status)
							return resp
						}
					}
				}
			}
		}
	}
	// Legacy/imported bindings can still use the device-bound public result endpoint.
	if binding != nil && strings.TrimSpace(binding.RedemptionToken) != "" {
		status, raw, err := cli.Result(ctx, binding.RedemptionToken, deviceID)
		if err == nil && status >= 200 && status < 300 {
			if order := publicLookupOrder(raw); order != nil {
				applyLookupOrder(&resp, order)
				applyLookupTimeline(&resp, raw)
			}
		}
	}
	return resp
}

func lookupOrderID(order map[string]any) int64 {
	if id := anyToInt64(order["order_id"]); id > 0 {
		return id
	}
	return anyToInt64(order["id"])
}

// Public results are normally {code:0,data:{order:{...},events:[]}}.
func publicLookupOrder(raw json.RawMessage) map[string]any {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return nil
	}
	if code, exists := body["code"]; exists && strAny(code) != "0" {
		return nil
	}
	if data, ok := body["data"].(map[string]any); ok {
		body = data
	}
	if order, ok := body["order"].(map[string]any); ok {
		return order
	}
	if strAny(body["status"]) != "" {
		return body
	}
	return nil
}

func applyLookupOrder(resp *cdkLookupResult, order map[string]any) {
	resp.OrderRequestID = strAny(order["client_request_id"])
	resp.OrderStatus = strings.ToLower(strings.TrimSpace(strAny(order["status"])))
	resp.OrderID = lookupOrderID(order)
	resp.Used, resp.CanResubmit = false, false
	resp.UsedAt = nil
	resp.FailureReason = ""
	resp.ErrorCode = publicMessage(order, "error_code")
	resp.Stage = publicMessage(order, "stage")
	resp.CreatedAt = strAny(order["created_at"])
	resp.UpdatedAt = strAny(order["updated_at"])
	if plan := strAny(order["plan"]); plan != "" {
		resp.Plan = plan
	}
	resp.AccountEmail = strAny(order["account_email"])
	switch resp.OrderStatus {
	case "completed":
		resp.Status, resp.Used = "used", true
		resp.Message = "兑换已完成"
		if resp.Plan == "pro_20x_renew" {
			resp.Message = "续费卡已绑定并设为默认；实际续费以到期扣款结果为准。"
		}
		if strings.HasPrefix(resp.Plan, "credit") {
			resp.Message = "点数加购已完成，请到账号中查看。"
		}
		if at := strAny(order["completed_at"]); at != "" && at != "0" {
			if seconds := anyToInt64(order["completed_at"]); seconds > 0 {
				at = time.Unix(seconds, 0).UTC().Format(time.RFC3339)
			}
			resp.UsedAt = &at
		}
	case "declined", "failed_precharge", "cancelled", "failed":
		reusable := strings.EqualFold(strings.TrimSpace(strAny(order["cdk_status"])), "unused")
		resp.FailureReason = publicMessage(order, "user_message", "message", "public_message", "failed_reason", "error_message")
		applyLookupFailure(resp, "", reusable)
		resp.Notes = resp.FailureReason
	case "queued", "awaiting_card", "funding_pending", "dispatching", "running", "requires_action", "pending", "plus_paid":
		resp.Status, resp.Message = "processing", "兑换处理中，请稍后查询，勿重复提交。"
	case "review":
		resp.Status, resp.Message = "review", "付款结果待对账，请等待确认，勿重复提交。"
	default:
		resp.Status, resp.Message = "unconfirmed", "尚未确认兑换结果，请稍后查询，勿重复提交。"
	}
}

func applyLookupLifecycle(resp *cdkLookupResult, status string) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "unused", "active":
		resp.Status, resp.Message = "unused", "卡密未使用，尚无已创建的兑换订单。"
	case "reserved":
		resp.Status, resp.Message = "processing", "卡密已预留，兑换结果待确认，勿重复提交。"
	case "review":
		resp.Status, resp.Message = "review", "付款结果待对账，请等待确认，勿重复提交。"
	case "frozen":
		resp.Status, resp.Message = "frozen", "卡密已冻结，请联系发码方。"
	case "disabled":
		resp.Status, resp.Message = "disabled", "卡密已禁用。"
	case "expired":
		resp.Status, resp.Message = "expired", "卡密已过期。"
		// consumed/used is not an order result; leave it unconfirmed without completed.
	}
}

// Compatibility for local CDKs: only a completed task confirms fulfillment.
func lookupLegacyCDK(code string) cdkLookupResult {
	resp := cdkLookupResult{CDKCode: code, Status: "unknown", Message: "未找到该卡密记录"}
	var status string
	var usedAt, expiresAt sql.NullTime
	err := db.DB.QueryRow(`SELECT COALESCE(plan_type,''), COALESCE(status,''), used_at, expires_at FROM cd_keys WHERE upper(trim(code))=upper(trim(?))`, code).Scan(&resp.Plan, &status, &usedAt, &expiresAt)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[lookup-cdk] local key read failed: %v", err)
			resp.Status = "unconfirmed"
			resp.Message = "暂无法查询卡密，请稍后重试。"
		}
		return resp
	}
	resp.Status, resp.Message = "unconfirmed", "尚未确认兑换结果，请稍后查询。"
	applyLookupLifecycle(&resp, status)
	reusable := resp.Status == "unused"
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		reusable = false
		if resp.Status == "unused" {
			resp.Status, resp.Message = "expired", "卡密已过期。"
		}
	}
	var task, email, notes string
	var completed sql.NullTime
	err = db.DB.QueryRow(`SELECT COALESCE(task_status,''), COALESCE(account_email,''), completed_at, COALESCE(notes,'') FROM recharge_tasks WHERE upper(trim(cdk_code))=upper(trim(?)) ORDER BY created_at DESC, rowid DESC LIMIT 1`, code).Scan(&task, &email, &completed, &notes)
	if err == nil {
		cdkStatus := ""
		if reusable {
			cdkStatus = "unused"
		}
		applyLookupOrder(&resp, map[string]any{"status": task, "account_email": email, "message": notes, "cdk_status": cdkStatus})
		if resp.Used && completed.Valid {
			at := completed.Time.UTC().Format(time.RFC3339)
			resp.UsedAt = &at
		}
	} else if err != sql.ErrNoRows {
		resp.Status, resp.Message = "unconfirmed", "暂无法核实兑换任务，请稍后查询，勿重复提交。"
	}
	return resp
}
