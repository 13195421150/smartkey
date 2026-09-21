package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

const aliasTestCode = "PLUS-TEST-CODE-EXAMPLE-0001"

func TestAliasQueryPreservesUpstreamStatusAndFailureDetails(t *testing.T) {
	setupLookupFixture(t, lookupFixture{orders: []map[string]any{fixtureOrder(12, "declined", "unused")}})
	err := db.SaveCDKAttempt(lookupTestCode, db.CDKAttemptDiagnostic{Phase: "preflight", Message: "模拟失败原因", ErrorCode: "TEST_DECLINED", StartedAt: 1, OccurredAt: "2026-09-21T01:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	native := lookupOneCDK(context.Background(), lookupTestCode, "test-device")
	alias := lookupOneCDK(context.Background(), aliasTestCode, "test-device")
	if alias.Status != native.Status || alias.Used != native.Used || alias.CDKCode != aliasTestCode || alias.LastAttempt == nil || alias.LastAttempt.Message != "模拟失败原因" {
		t.Fatalf("alias status/diagnostics mismatch: %+v", alias)
	}
	wrong := lookupOneCDK(context.Background(), "Pro20X-TEST-CODE-EXAMPLE-0001", "test-device")
	if wrong.Status != "unknown" || wrong.OrderID != 0 {
		t.Fatal("wrong plan exposed an order")
	}
}

func TestOriginalAndAliasBothRejectConsumedCode(t *testing.T) {
	setupLookupFixture(t, lookupFixture{current: "consumed"})
	for _, code := range []string{lookupTestCode, aliasTestCode} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"code":"`+code+`"}`))
		PublicCDKPreview(c)
		if w.Code != 400 || !strings.Contains(w.Body.String(), usedCDKMessage) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestAliasPreviewBindsOnlyCanonicalCodeAndRejectsWrongPlan(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["code"] != lookupTestCode {
			t.Errorf("upstream received noncanonical code")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"data":{"plan":"plus","redemption_token":"alias-test-token"}}`))
	}))
	defer upstream.Close()
	db.SetSetting("card_api_base", upstream.URL)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"code":"`+aliasTestCode+`"}`))
	PublicCDKPreview(c)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	canonical, err := db.FindCodeByRedemptionToken("alias-test-token")
	if err != nil || canonical != lookupTestCode {
		t.Fatal(canonical, err)
	}
	var count int
	db.DB.QueryRow("SELECT COUNT(*) FROM cdk_session_bindings").Scan(&count)
	if count != 1 {
		t.Fatal("split alias binding")
	}
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"code":"Pro20X-TEST-CODE-EXAMPLE-0001"}`))
	PublicCDKPreview(c)
	if w.Code != 400 || calls != 1 {
		t.Fatal("wrong-plan alias reached upstream")
	}
}

func TestAliasResultAndBatchQueryUseOriginalBinding(t *testing.T) {
	setupLookupFixture(t, lookupFixture{current: "unused", result: `{"code":0,"data":{"status":"declined"}}`})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?code="+aliasTestCode, nil)
	PublicCDKResultByCode(c)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"cdk_code":"`+aliasTestCode+`"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"codes":["`+lookupTestCode+`","`+aliasTestCode+`"]}`))
	LookupCDKStatusBatch(c)
	var body struct {
		Results []cdkLookupResult `json:"results"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 200 || len(body.Results) != 2 || body.Results[0].Status != body.Results[1].Status {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestStoredExportUsesAliasesWithoutChangingDatabase(t *testing.T) {
	setupLookupFixture(t, lookupFixture{})
	db.DB.Exec("ALTER TABLE cardplatform_cdk_codes ADD COLUMN fee_amount_minor INTEGER DEFAULT 0")
	db.DB.Exec("CREATE TABLE cardplatform_cdk_notes(upstream_id INTEGER,note TEXT)")
	for _, format := range []string{"txt", "json"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/?sync=0&format="+format+"&q=PLUS-", nil)
		CardPlatformListStoredCDKs(c)
		if w.Code != 200 || !strings.Contains(w.Body.String(), aliasTestCode) || strings.Contains(w.Body.String(), lookupTestCode) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	raw, _ := db.LookupCardplatformCDKCode(7, "")
	if raw != lookupTestCode {
		t.Fatal("upstream code overwritten")
	}
}
