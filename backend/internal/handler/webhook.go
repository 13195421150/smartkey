package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

// CardPlatformWebhook POST /api/v1/webhooks/cardplatform
// 卡台开发者页配置的回调地址指向此处。
// 验签：X-Signature = hex(HMAC-SHA256(webhook_secret, raw body))
// 文档：zovocard-openapi-zh.md §7
func CardPlatformWebhook(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, (1<<20)+1))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if len(raw) > 1<<20 {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}
	secret, _ := db.GetSetting("webhook_secret")
	secret = strings.TrimSpace(secret)
	if secret == "" {
		// 未配置密钥时拒绝，避免裸奔
		log.Printf("webhook: webhook_secret not configured")
		c.Status(http.StatusServiceUnavailable)
		return
	}
	got := strings.TrimSpace(c.GetHeader("X-Signature"))
	if got == "" {
		c.Status(http.StatusUnauthorized)
		return
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	expect := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(got)), []byte(strings.ToLower(expect))) != 1 {
		c.Status(http.StatusUnauthorized)
		return
	}

	// Acknowledge only a valid event that can be stored or was already received.
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil || payload == nil {
		c.Status(http.StatusBadRequest)
		return
	}

	eventType := ""
	if v, ok := payload["event"].(string); ok {
		eventType = v
	}
	if v, ok := payload["type"].(string); ok && eventType == "" {
		eventType = v
	}
	if eventType == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	// 幂等键
	idem := webhookIdemKey(payload, eventType)
	if err := db.InsertWebhookEvent(eventType, idem, string(raw)); err != nil {
		// 唯一冲突视为已处理（幂等）
		if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(err.Error(), "unique") {
			log.Printf("webhook store: %v", err)
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusOK)
		return
	} else {
		// 若是 GPT 直充完成，可写审计
		if eventType == "gpt_direct.completed" {
			db.WriteAudit("webhook", "gpt_direct.completed", idem, c.ClientIP())
		}
	}
	// 本站 CDK 状态：兑换完成 → consumed（避免列表仍显示「未使用」）
	if strings.HasPrefix(strings.ToLower(eventType), "gpt_direct.") || strings.HasPrefix(strings.ToLower(eventType), "cdk.") {
		applyCDKStatusFromWebhook(payload, eventType)
	}
	// 卡健康：失败/成功终态观察（同卡多邮箱失败 → 拉黑）
	if strings.HasPrefix(strings.ToLower(eventType), "gpt_direct.") {
		observeFromWebhookPayload(payload)
	}
	c.Status(http.StatusOK)
}

// applyCDKStatusFromWebhook 根据卡台终态回写本站 SQLite 中的 CDK status。
func applyCDKStatusFromWebhook(payload map[string]interface{}, eventType string) {
	if payload == nil {
		return
	}
	cdkID := anyToInt64(payload["cdk_id"])
	if cdkID <= 0 {
		return
	}
	et := strings.ToLower(strings.TrimSpace(eventType))
	st := strings.ToLower(strings.TrimSpace(strAny(payload["cdk_status"])))
	if st == "" {
		switch et {
		case "gpt_direct.completed", "cdk.consumed":
			st = "consumed"
		case "cdk.reserved":
			st = "reserved"
		case "cdk.released", "cdk.unfrozen":
			st = "unused"
		case "cdk.frozen":
			st = "frozen"
		case "cdk.disabled":
			st = "disabled"
		default:
			return
		}
	}
	switch st {
	case "unused", "reserved", "consumed", "frozen", "disabled", "review":
	default:
		return
	}
	if db.GetCardplatformCDKStatus(cdkID) == "consumed" && st != "consumed" {
		return
	}
	// 失败回传 unused 时，勿把已 consumed/disabled 降级回去
	if st == "unused" || st == "reserved" {
		if cur := db.GetCardplatformCDKStatus(cdkID); cur == "consumed" || cur == "disabled" {
			return
		}
	}
	_ = db.UpdateCardplatformCDKStatus(cdkID, st)
}

func webhookIdemKey(p map[string]interface{}, eventType string) string {
	if id := strings.TrimSpace(strAny(p["event_id"])); id != "" {
		return "event|" + id
	}
	str := func(k string) string {
		if v, ok := p[k]; ok {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return strings.TrimSuffix(strings.TrimSuffix(jsonNumber(t), ".0"), ".")
			}
		}
		return ""
	}
	switch eventType {
	case "card_transaction":
		return strings.Join([]string{eventType, str("auth_id"), str("type"), str("status")}, "|")
	case "card_operation":
		return strings.Join([]string{eventType, str("operation"), str("operation_id"), str("status")}, "|")
	case "gpt_direct.completed":
		if id := str("order_id"); id != "" {
			return "gpt_direct.completed|order|" + id
		}
		if id := str("client_request_id"); id != "" {
			return "gpt_direct.completed|client|" + id
		}
	}
	// fallback
	// Stable fallback for older senders without event_id; map keys marshal in order.
	raw, _ := json.Marshal(p)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:16])
}

func jsonNumber(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// AdminListWebhooks GET /api/v1/admin/webhooks/events
func AdminListWebhooks(c *gin.Context) {
	limit := 50
	rows, err := db.ListWebhookEvents(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	// 脱敏：不返回完整 card_number
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":         r.ID,
			"event_type": r.EventType,
			"idem_key":   r.IdemKey,
			"created_at": r.CreatedAt,
			"payload":    sanitizeWebhookPayload(r.Payload),
		})
	}
	urlHint := ""
	if host := c.Request.Host; host != "" {
		scheme := "https"
		if c.Request.TLS == nil && !strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			// 生产经 Caddy 会带 proto
			if p := c.GetHeader("X-Forwarded-Proto"); p != "" {
				scheme = p
			}
		} else if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		urlHint = scheme + "://" + host + "/api/v1/webhooks/cardplatform"
	}
	sec, _ := db.GetSetting("webhook_secret")
	c.JSON(http.StatusOK, gin.H{
		"events":              out,
		"webhook_url":         urlHint,
		"webhook_secret_set":  strings.TrimSpace(sec) != "",
		"webhook_secret_hint": maskSecret(sec),
	})
}

func sanitizeWebhookPayload(raw string) interface{} {
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) != nil {
		return raw
	}
	if cn, ok := m["card_number"].(string); ok && len(cn) > 4 {
		m["card_number"] = "****" + cn[len(cn)-4:]
	}
	return m
}
