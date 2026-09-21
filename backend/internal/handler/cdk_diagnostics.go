package handler

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tuzi/cdk-recharge-system/internal/cardplatform"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

const usedCDKMessage = "卡密无效CDK已被使用"

func confirmedConsumedCDK(ctx context.Context, code string) bool {
	id, _, _, _, known := db.LookupStoredCDKByCode(code)
	if !known || id <= 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	list, err := cardplatform.NewFromSettings().ListCDKsQuery(ctx, cardplatform.CDKListQuery{Page: 1, PageSize: 100, Query: strconv.FormatInt(id, 10)})
	if err != nil || list == nil {
		return false
	}
	for _, item := range list.List {
		if item.ID == id && strings.EqualFold(item.Status, "consumed") {
			return true
		}
	}
	return false
}

func publicResponseFailed(status int, raw json.RawMessage) bool {
	if status >= 400 {
		return true
	}
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return false
	}
	code, exists := body["code"]
	return exists && strAny(code) != "0"
}

// Only bounded public text is retained. Request credentials and raw error objects are excluded.
var diagnosticTokenPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_.-]+|\b(?:sk_|whsec_)[A-Za-z0-9_-]+|\b[0-9]{12,19}\b`)

func diagnosticText(s string) string {
	s = diagnosticTokenPattern.ReplaceAllString(strings.TrimSpace(s), "[已隐藏敏感信息]")
	r := []rune(s)
	if len(r) > 2000 {
		s = string(r[:2000]) + "…"
	}
	return s
}
func publicMessage(m map[string]any, keys ...string) string {
	return diagnosticText(messageText(m, keys...))
}
func messageText(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func recordCDKAttempt(token, phase string, request map[string]any, status int, raw json.RawMessage, requestErr error, started time.Time) {
	// Associate the outcome using the server's token binding, never a client-supplied code.
	code, err := db.FindCodeByRedemptionToken(token)
	if err != nil || code == "" {
		return
	}
	d := db.CDKAttemptDiagnostic{Phase: phase, RequestID: str(request["client_request_id"]), StartedAt: started.UnixNano(), OccurredAt: time.Now().UTC().Format(time.RFC3339)}
	if requestErr != nil {
		d.Message = "与卡台连接中断，结果尚未确认，请稍后查询。"
		d.Uncertain = true
	} else if publicResponseFailed(status, raw) {
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		d.Message = messageText(body, "msg", "message", "error")
		d.ErrorCode = publicMessage(body, "error_code")
		if d.Message == "" {
			d.Message = "卡台未接受本次请求，未提供进一步原因。"
		}
		d.Uncertain = status >= 500 || status == 408 || status == 409
	}
	// Remove any submitted credential values even if an upstream error echoed them.
	if credential, ok := request["credential"].(map[string]any); ok {
		for _, key := range []string{"session", "password", "access_token"} {
			if secret, ok := credential[key].(string); ok && len(secret) > 0 {
				d.Message = strings.ReplaceAll(d.Message, secret, "[已隐藏敏感信息]")
				var session map[string]any
				if key == "session" && json.Unmarshal([]byte(secret), &session) == nil {
					for _, k := range []string{"sessionToken", "session_token", "accessToken", "access_token", "refreshToken"} {
						if v, ok := session[k].(string); ok && len(v) >= 8 {
							d.Message = strings.ReplaceAll(d.Message, v, "[已隐藏敏感信息]")
						}
					}
				}
			}
		}
	}
	d.Message = diagnosticText(d.Message)
	if err := db.SaveCDKAttempt(code, d); err != nil {
		log.Printf("[cdk-diagnostic] save failed: %v", err)
	}
}

func attachCDKAttempt(resp *cdkLookupResult) {
	if resp.OrderStatus == "completed" {
		return
	}
	d, err := db.GetCDKAttempt(resp.CDKCode)
	if err != nil || d == nil {
		return
	}
	if d.Phase == "redeem" && d.Uncertain && d.RequestID != "" && d.RequestID == resp.OrderRequestID {
		return
	}
	resp.LastAttempt = d
	if d.Phase == "redeem" && d.Uncertain {
		resp.Status = "unconfirmed"
		resp.CanResubmit = false
		resp.Message = "最近一次提交结果尚未确认，请稍后查询，勿重复提交；历史订单不能作为重试依据。"
	}
}
