package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

func rotationRequest(t *testing.T, request string, generation int64) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"client_request_id": request, "generation": generation})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(string(body)))
	CardPlatformRotateCDK(c)
	return w
}
func rotatedCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["id"] != float64(7) || body["generation"] != float64(1) {
		t.Fatal(body)
	}
	return body["code"].(string)
}
func publicRequest(method, query, body string, f gin.HandlerFunc) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/"+query, strings.NewReader(body))
	f(c)
	return w
}

func TestSiteRotationPreservesMappingAndHistoryAndRevokesEveryOldCode(t *testing.T) {
	setupLookupFixture(t, lookupFixture{orders: []map[string]any{fixtureOrder(12, "declined", "unused")}})
	if err := db.SaveCDKAttempt(lookupTestCode, db.CDKAttemptDiagnostic{Phase: "redeem", Message: "原失败原因", StartedAt: 1, OccurredAt: "2026-10-09T01:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	db.DB.Exec(`INSERT INTO recharge_tasks VALUES(?,'failed','fixture@example.test',NULL,'原任务',CURRENT_TIMESTAMP)`, lookupTestCode)
	before := lookupOneCDK(context.Background(), aliasTestCode, "test")
	newCode := rotatedCode(t, rotationRequest(t, "synthetic-request-0001", 0))
	if !strings.HasPrefix(newCode, "PLUS-") || newCode == aliasTestCode {
		t.Fatal(newCode)
	}
	if code, err := db.ResolveCDKCode(newCode); err != nil || code != lookupTestCode {
		t.Fatal(code, err)
	}
	for _, old := range []string{lookupTestCode, aliasTestCode, strings.ToLower(aliasTestCode)} {
		if _, err := db.ResolveCDKCode(old); !errors.Is(err, db.ErrCDKAlias) {
			t.Fatal("old key valid", err)
		}
		preview := publicRequest("POST", "", `{"code":"`+old+`"}`, PublicCDKPreview)
		result := publicRequest("GET", "?code="+old, "", PublicCDKResultByCode)
		if preview.Code != 400 || result.Code != 400 || !strings.Contains(result.Body.String(), "CDK_INVALID") {
			t.Fatal("old key was accepted")
		}

	}
	after := lookupOneCDK(context.Background(), newCode, "test")
	if after.OrderID != before.OrderID || after.Status != before.Status || after.Used != before.Used || after.LastAttempt == nil || after.LastAttempt.Message != "原失败原因" {
		t.Fatalf("history changed: %+v %+v", before, after)
	}
	var count int
	db.DB.QueryRow(`SELECT COUNT(*) FROM recharge_tasks WHERE cdk_code=? AND notes='原任务' AND task_status='failed'`, lookupTestCode).Scan(&count)
	if count != 1 {
		t.Fatal("task modified")
	}
	canonical, _ := db.LookupCardplatformCDKCode(7, "")
	if canonical != lookupTestCode {
		t.Fatal("upstream mapping modified")
	}
	db.DB.Exec(`ALTER TABLE cardplatform_cdk_codes ADD COLUMN fee_amount_minor INTEGER DEFAULT 0; CREATE TABLE cardplatform_cdk_notes(upstream_id INTEGER,note TEXT)`)
	for _, format := range []string{"txt", "json"} {
		w := publicRequest("GET", "?sync=0&format="+format+"&q="+newCode, "", CardPlatformListStoredCDKs)
		if w.Code != 200 || !strings.Contains(w.Body.String(), newCode) || strings.Contains(w.Body.String(), aliasTestCode) || strings.Contains(w.Body.String(), lookupTestCode) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	// Re-synchronization and stale browser cache imports cannot recreate the site key.
	for _, code := range []string{lookupTestCode, aliasTestCode, newCode} {
		if err := db.SaveCardplatformCDKCode(7, code, "", "plus", 15); err != nil {
			t.Fatal(err)
		}
	}
	site, _ := db.SiteCDKFor(lookupTestCode, "plus")
	if site.Code != newCode || site.Generation != 1 {
		t.Fatal("sync replaced site mapping")
	}
	db.DB.QueryRow(`SELECT COUNT(*) FROM cardplatform_cdk_codes`).Scan(&count)
	if count != 1 {
		t.Fatal("new upstream CDK inserted")
	}
	// An idempotent retry does not rotate twice; stale and old requests cannot overwrite it.
	if got := rotatedCode(t, rotationRequest(t, "synthetic-request-0001", 0)); got != newCode {
		t.Fatal("retry created another code")
	}
	if w := rotationRequest(t, "synthetic-request-0002", 0); w.Code != 409 {
		t.Fatal(w.Code)
	}
	w := rotationRequest(t, "synthetic-request-0002", 1)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.ResolveCDKCode(newCode); !errors.Is(err, db.ErrCDKAlias) {
		t.Fatal("previous random key valid")
	}
	if err := db.SaveCardplatformCDKCode(7, newCode, "", "plus", 15); err != nil {
		t.Fatal(err)
	}
	db.DB.QueryRow(`SELECT COUNT(*) FROM cardplatform_cdk_codes`).Scan(&count)
	if count != 1 {
		t.Fatal("retired cache created phantom CDK")
	}
}

func TestRotationRevokesLegacyAndSupersededTokensBeforeAnyUpstreamMutation(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	for _, tok := range []string{"first-preview", "second-preview"} {
		if err := db.BindCDKRedemptionToken(lookupTestCode, tok); err != nil {
			t.Fatal(err)
		}
	}
	rotatedCode(t, rotationRequest(t, "synthetic-request-0001", 0))
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer upstream.Close()
	db.SetSetting("card_api_base", upstream.URL)
	for _, tok := range []string{"fixture-token", "first-preview", "second-preview", "unknown-token"} {
		for _, f := range []gin.HandlerFunc{PublicCDKPreflight, PublicCDKRedeem, PublicCDKGraceRecovery} {
			w := publicRequest("POST", "", `{"redemption_token":"`+tok+`","preflight_token":"preflight-test","confirmed":true}`, f)
			if w.Code != 400 || !strings.Contains(w.Body.String(), "CDK_SESSION_REVOKED") {
				t.Fatal(w.Code, w.Body.String())
			}
		}
		w := publicRequest("GET", "?token="+tok, "", PublicCDKResult)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls != 0 {
		t.Fatal("revoked token reached upstream")
	}
	if err := db.BindCDKRedemptionToken(lookupTestCode, "new-preview"); err != nil {
		t.Fatal(err)
	}
	if code, err := db.ValidateSiteCDKToken("new-preview"); err != nil || code != lookupTestCode {
		t.Fatal(code, err)
	}
}

func TestRotationChecksRealtimeStatusAndActiveOrders(t *testing.T) {
	for _, status := range []string{"consumed", "reserved", "review", "frozen", "disabled"} {
		t.Run(status, func(t *testing.T) {
			setupLookupFixture(t, lookupFixture{current: status})
			if w := rotationRequest(t, "synthetic-request-0001", 0); w.Code != 409 {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	for _, status := range []string{"pending", "processing", "completed", "unknown"} {
		t.Run("order-"+status, func(t *testing.T) {
			setupLookupFixture(t, lookupFixture{orders: []map[string]any{fixtureOrder(12, status, "unused")}})
			if w := rotationRequest(t, "synthetic-request-0001", 0); w.Code != 409 {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	t.Run("outage", func(t *testing.T) {
		setupLookupFixture(t, lookupFixture{ordersHTTP: 503})
		if w := rotationRequest(t, "synthetic-request-0001", 0); w.Code < 400 {
			t.Fatal(w.Code)
		}
	})
	t.Run("accepted-not-in-list", func(t *testing.T) {
		setupLookupFixture(t, lookupFixture{})
		db.MarkSiteCDKSubmitted(lookupTestCode, "fixture-token")
		if w := rotationRequest(t, "synthetic-request-0001", 0); w.Code != 409 {
			t.Fatal(w.Code, w.Body.String())
		}
	})
	t.Run("confirmed-failure", func(t *testing.T) {
		setupLookupFixture(t, lookupFixture{result: `{"code":0,"data":{"status":"declined"}}`})
		db.MarkSiteCDKSubmitted(lookupTestCode, "fixture-token")
		rotatedCode(t, rotationRequest(t, "synthetic-request-0001", 0))
	})
}

func TestRotationWaitsForInFlightCodeLockAndRechecksToken(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	unlock := lockSiteCDK(lookupTestCode)
	finished := make(chan *httptest.ResponseRecorder, 1)
	go func() { finished <- rotationRequest(t, "synthetic-request-0001", 0) }()
	select {
	case <-finished:
		t.Fatal("rotation did not wait for redemption lock")
	case <-time.After(30 * time.Millisecond):
	}
	unlock()
	select {
	case w := <-finished:
		rotatedCode(t, w)
	case <-time.After(2 * time.Second):
		t.Fatal("rotation lock was not released")
	}
	if _, err := db.ValidateSiteCDKToken("fixture-token"); !errors.Is(err, db.ErrCDKSessionRevoked) {
		t.Fatal("legacy token not revoked")
	}
}

func TestOpaqueTokensWorkAcrossPreviewPreflightRedeemAndUpstreamSessionReuse(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	db.DB.Exec(`CREATE TABLE card_blocklist(card_id INTEGER,unblocked_at TEXT)`)
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && r.URL.Path == "/openapi/v1/gpt-direct/cdks" {
			w.Write([]byte(`{"code":0,"data":{"list":[{"id":7,"plan":"plus","status":"unused"}],"total":1}}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/openapi/v1/gpt-direct/cdk-orders" {
			w.Write([]byte(`{"code":0,"data":{"list":[],"total":0}}`))
			return
		}
		if r.URL.Path == "/api/v1/cdk/preview" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["code"] != lookupTestCode {
				t.Error("mapped CDK did not reach upstream")
			}
			w.Write([]byte(`{"code":0,"data":{"plan":"plus","redemption_token":"reused-upstream-token"}}`))
			return
		}
		token := r.URL.Query().Get("token")
		if r.Method == "POST" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			token = str(body["redemption_token"])
		}
		if token != "reused-upstream-token" {
			t.Errorf("opaque token leaked upstream: %s", token)
		}
		calls++
		w.Write([]byte(`{"code":0,"data":{"status":"declined","redemption_token":"reused-upstream-token"}}`))
	}))
	defer upstream.Close()
	db.SetSetting("card_api_base", upstream.URL)
	preview := func(code string) string {
		w := publicRequest("POST", "", `{"code":"`+code+`"}`, PublicCDKPreview)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		token := extractJSONNestedString(w.Body.Bytes(), "data", "redemption_token")
		if !strings.HasPrefix(token, "scdk_") {
			t.Fatal("missing opaque token")
		}
		return token
	}
	first := preview(aliasTestCode)
	for _, f := range []gin.HandlerFunc{PublicCDKPreflight, PublicCDKRedeem, PublicCDKGraceRecovery} {
		w := publicRequest("POST", "", `{"redemption_token":"`+first+`","preflight_token":"synthetic-preflight","confirmed":true}`, f)
		if w.Code != 200 || strings.Contains(w.Body.String(), "reused-upstream-token") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := publicRequest("GET", "?token="+first, "", PublicCDKResult); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	next := rotatedCode(t, rotationRequest(t, "synthetic-request-0001", 0))
	second := preview(next)
	if first == second {
		t.Fatal("site token was reused")
	}
	for _, token := range []string{first, "reused-upstream-token"} {
		w := publicRequest("POST", "", `{"redemption_token":"`+token+`"}`, PublicCDKPreflight)
		if w.Code != 400 {
			t.Fatal("old session restored after new preview", w.Code, w.Body.String())
		}
	}
	w := publicRequest("POST", "", `{"redemption_token":"`+second+`"}`, PublicCDKPreflight)
	if w.Code != 200 {
		t.Fatal("new session failed", w.Code, w.Body.String())
	}
	if calls != 5 {
		t.Fatal("unexpected upstream requests", calls)
	}
}

func TestRotationRejectsIncompleteOrderResponse(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/openapi/v1/gpt-direct/cdks" {
			w.Write([]byte(`{"code":0,"data":{"list":[{"id":7,"status":"unused"}],"total":1}}`))
			return
		}
		w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer upstream.Close()
	db.SetSetting("card_api_base", upstream.URL)
	if w := rotationRequest(t, "synthetic-request-0001", 0); w.Code != 503 {
		t.Fatal(w.Code, w.Body.String())
	}
	current, _ := db.SiteCDKFor(lookupTestCode, "plus")
	if current.Generation != 0 {
		t.Fatal("incomplete response rotated key")
	}
}

func TestRotationReconcilesSubmittedTokenWithItsOriginalDevice(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	if err := db.BindCDKRedemptionToken(lookupTestCode, "site-token", "upstream-token", "customer-device"); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkSiteCDKSubmitted(lookupTestCode, "site-token"); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/openapi/v1/gpt-direct/cdks":
			w.Write([]byte(`{"code":0,"data":{"list":[{"id":7,"status":"unused"}],"total":1}}`))
		case "/openapi/v1/gpt-direct/cdk-orders":
			w.Write([]byte(`{"code":0,"data":{"list":[],"total":0}}`))
		case "/api/v1/cdk/result":
			if r.Header.Get("X-Redemption-Device") != "customer-device" {
				w.WriteHeader(403)
				return
			}
			if r.URL.Query().Get("token") != "upstream-token" {
				t.Error("wrong upstream token")
			}
			w.Write([]byte(`{"code":0,"data":{"status":"declined"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	db.SetSetting("card_api_base", upstream.URL)
	rotatedCode(t, rotationRequest(t, "synthetic-request-0001", 0))
}
