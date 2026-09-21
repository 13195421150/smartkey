package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

func TestUsedPreviewRequiresFreshConsumedState(t *testing.T) {
	for _, status := range []string{"consumed", "disabled", "unused", "reserved"} {
		t.Run(status, func(t *testing.T) {
			setupLookupFixture(t, lookupFixture{stored: "consumed", current: status})
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("POST", "/preview", strings.NewReader(`{"code":"`+lookupTestCode+`"}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			PublicCDKPreview(ctx)
			if w.Code != 400 {
				t.Fatalf("unexpected preview status %d", w.Code)
			}
			if strings.Contains(w.Body.String(), usedCDKMessage) != (status == "consumed") {
				t.Fatalf("incorrect used-state message: %s", w.Body)
			}
		})
	}
}

func TestLookupReturnsScopedFailureDetailAndPublicTimeline(t *testing.T) {
	setupLookupFixture(t, lookupFixture{orders: []map[string]any{fixtureOrder(12, "declined", "unused")}, detail: `{"order":{"order_id":12,"cdk_id":7,"status":"declined","cdk_status":"unused","user_message":"卡片余额不足，请联系发码方补款","error_code":"INSUFFICIENT_CARD_BALANCE","stage":"payment","created_at":"2026-09-20T09:00:00Z","credential":"private-fixture-value"},"events":[{"created_at":"2026-09-20T09:01:00Z","to_status":"declined","step":"payment","public_code":"PAYMENT_DECLINED","public_message":"支付被拒：余额不足","internal_error":"internal-fixture-value"}]}`})
	r := lookupOneCDK(context.Background(), lookupTestCode, "another-device")
	if r.Status != "failed" || !r.DetailsAvailable || r.FailureReason != "卡片余额不足，请联系发码方补款" || len(r.Events) != 1 || r.ErrorCode != "INSUFFICIENT_CARD_BALANCE" {
		t.Fatalf("missing failure detail: %+v", r)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "private-fixture-value") || strings.Contains(string(b), "internal-fixture-value") {
		t.Fatal("private upstream fields leaked")
	}
}

func TestLookupRejectsUnrelatedDetail(t *testing.T) {
	setupLookupFixture(t, lookupFixture{orders: []map[string]any{fixtureOrder(12, "declined", "unused")}, detail: `{"order":{"order_id":12,"cdk_id":999,"status":"completed","user_message":"foreign-customer-message"}}`})
	r := lookupOneCDK(context.Background(), lookupTestCode, "device")
	if r.Status != "failed" || r.DetailsAvailable || strings.Contains(r.FailureReason, "foreign") {
		t.Fatalf("unrelated detail accepted: %+v", r)
	}
}

func TestPreflightFailureIsVisibleWithoutOrder(t *testing.T) {
	setupLookupFixture(t, lookupFixture{preflight: `{"code":400,"error_code":"GPT_SESSION_INVALID","msg":"登录已过期：secret-session-fixture-value"}`})
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("POST", "/preflight", strings.NewReader(`{"code":"WRONG-CLIENT-CODE","redemption_token":"fixture-token","credential":{"mode":"session","session":"{\"sessionToken\":\"secret-session-fixture-value\"}"}}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	PublicCDKPreflight(ctx)
	r := lookupOneCDK(context.Background(), lookupTestCode, "device")
	if r.Status != "unused" || r.LastAttempt == nil || r.LastAttempt.ErrorCode != "GPT_SESSION_INVALID" {
		t.Fatalf("missing validation failure: %+v", r)
	}
	if !strings.Contains(r.LastAttempt.Message, "登录已过期") || strings.Contains(r.LastAttempt.Message, "secret-session-fixture-value") {
		t.Fatal("error message missing or credential leaked")
	}
	wrong, _ := db.GetCDKAttempt("WRONG-CLIENT-CODE")
	if wrong != nil {
		t.Fatal("client poisoned another CDK")
	}
	start := time.Now().Add(time.Second)
	recordCDKAttempt("fixture-token", "preflight", nil, 200, json.RawMessage(`{"code":0}`), nil, start)
	recordCDKAttempt("fixture-token", "preflight", nil, 400, json.RawMessage(`{"code":400,"msg":"stale failure"}`), nil, start.Add(-time.Second))
	if d, _ := db.GetCDKAttempt(lookupTestCode); d != nil {
		t.Fatal("late failure replaced newer success")
	}
}

func TestUncertainSubmissionCannotOfferAnotherRedemption(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	recordCDKAttempt("fixture-token", "redeem", nil, 503, json.RawMessage(`{"code":503,"msg":"上游连接超时"}`), nil, time.Now())
	r := lookupOneCDK(context.Background(), lookupTestCode, "device")
	if r.Status != "unconfirmed" || r.CanResubmit || r.LastAttempt == nil || !r.LastAttempt.Uncertain {
		t.Fatalf("unsafe retry after uncertainty: %+v", r)
	}
}

func TestOldFailedOrderCannotResolveNewSubmissionUncertainty(t *testing.T) {
	old := fixtureOrder(12, "declined", "unused")
	old["client_request_id"] = "old-request"
	setupLookupFixture(t, lookupFixture{orders: []map[string]any{old}})
	recordCDKAttempt("fixture-token", "redeem", map[string]any{"client_request_id": "new-request"}, 503, json.RawMessage(`{"code":503,"msg":"timeout"}`), nil, time.Now())
	r := lookupOneCDK(context.Background(), lookupTestCode, "device")
	if r.Status != "unconfirmed" || r.CanResubmit {
		t.Fatal("old failed order enabled another submission")
	}
	old["client_request_id"] = "new-request"
	r = lookupOneCDK(context.Background(), lookupTestCode, "device")
	if r.Status != "failed" || !r.CanResubmit || r.LastAttempt != nil {
		t.Fatal("matching reconciled order did not resolve uncertainty")
	}
}
