package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

const lookupTestCode = "ZC-TEST-CODE-EXAMPLE-0001"

type lookupFixture struct {
	stored, current string
	orders          []map[string]any
	ordersHTTP      int
	result          string
	detail          string
	preflight       string
}

func setupLookupFixture(t *testing.T, f lookupFixture) {
	t.Helper()
	old := db.DB
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	db.DB = conn
	t.Cleanup(func() { conn.Close(); db.DB = old })
	for _, q := range []string{
		`CREATE TABLE site_settings (key TEXT PRIMARY KEY, value TEXT, updated_at DATETIME)`,
		`CREATE TABLE cd_keys (code TEXT, plan_type TEXT, status TEXT, used_at DATETIME, expires_at DATETIME)`,
		`CREATE TABLE cardplatform_cdk_codes (upstream_id INTEGER, code TEXT, code_prefix TEXT, plan TEXT, status TEXT, created_at DATETIME)`,
		`CREATE TABLE recharge_tasks (cdk_code TEXT, task_status TEXT, account_email TEXT, completed_at DATETIME, notes TEXT, created_at DATETIME)`,
		`CREATE TABLE cdk_session_bindings (cdk_code TEXT PRIMARY KEY, redemption_token TEXT, session_payload TEXT, updated_at TEXT)`,
		`CREATE TABLE cdk_attempt_diagnostics(cdk_code TEXT PRIMARY KEY,phase TEXT,request_id TEXT,error_code TEXT,message TEXT,uncertain INTEGER,started_at INTEGER,occurred_at TEXT)`,
	} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if f.stored == "" {
		f.stored = "unused"
	}
	if f.current == "" {
		f.current = "unused"
	}
	if _, err := conn.Exec(`INSERT INTO cardplatform_cdk_codes VALUES(7, ?, 'ZC-TEST-CODE', 'plus', ?, CURRENT_TIMESTAMP)`, lookupTestCode, f.stored); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO cdk_session_bindings VALUES(?, 'fixture-token', '{"user":{"email":"fixture@example.test"}}', '')`, lookupTestCode); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/openapi/v1/gpt-direct/cdk-orders":
			if r.URL.Query().Get("cdk_id") != "7" || r.Header.Get("X-API-Key") != "fixture-api-key" {
				t.Error("missing scoped reconciliation/authentication")
			}
			if f.ordersHTTP != 0 {
				w.WriteHeader(f.ordersHTTP)
				w.Write([]byte(`{"code":503}`))
				return
			}
			orders := f.orders
			if orders == nil {
				orders = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"list": orders, "total": len(orders)}})
		case "/openapi/v1/gpt-direct/cdks":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"list": []map[string]any{{"id": 7, "code": lookupTestCode, "status": f.current, "plan": "plus"}}, "total": 1}})
		case "/api/v1/cdk/result":
			if f.result == "" {
				w.WriteHeader(404)
				w.Write([]byte(`{"code":404}`))
				return
			}
			w.Write([]byte(f.result))
		case "/openapi/v1/gpt-direct/cdk-orders/12":
			if f.detail == "" {
				w.WriteHeader(404)
				return
			}
			w.Write([]byte(`{"code":0,"data":` + f.detail + `}`))
		case "/api/v1/cdk/preview":
			w.WriteHeader(400)
			w.Write([]byte(`{"code":400,"msg":"CDK 无效或不可用"}`))
		case "/api/v1/cdk/preflight":
			w.WriteHeader(400)
			w.Write([]byte(f.preflight))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	if err := db.SetSetting("card_api_base", srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting("card_api_key", "fixture-api-key"); err != nil {
		t.Fatal(err)
	}
}

func fixtureOrder(id int, status, cdkStatus string) map[string]any {
	return map[string]any{"order_id": id, "cdk_id": 7, "status": status, "cdk_status": cdkStatus, "plan": "plus"}
}

func TestLookupUsesOrderFacts(t *testing.T) {
	cases := []struct {
		name        string
		f           lookupFixture
		status      string
		used, retry bool
	}{
		{"session binding is not success", lookupFixture{}, "unused", false, false},
		{"failure overrides stale consumed cache", lookupFixture{stored: "consumed", orders: []map[string]any{fixtureOrder(12, "failed_precharge", "unused")}}, "failed", false, true},
		{"declined but not released cannot retry", lookupFixture{current: "reserved", orders: []map[string]any{fixtureOrder(12, "declined", "reserved")}}, "failed", false, false},
		{"latest attempt wins over older success", lookupFixture{orders: []map[string]any{fixtureOrder(9, "completed", "consumed"), fixtureOrder(12, "cancelled", "unused")}}, "failed", false, true},
		{"confirmed success without usable public token", lookupFixture{orders: []map[string]any{fixtureOrder(12, "completed", "consumed")}}, "used", true, false},
		{"consumed alone is not successful order", lookupFixture{stored: "consumed", current: "consumed"}, "unconfirmed", false, false},
		{"outage cannot reuse cached success", lookupFixture{stored: "consumed", ordersHTTP: 503}, "unconfirmed", false, false},
		{"nested public failure is parsed", lookupFixture{ordersHTTP: 503, result: `{"code":0,"data":{"order":{"status":"failed_precharge","cdk_status":"unused"}}}`}, "failed", false, true},
		{"business error cannot masquerade as success", lookupFixture{ordersHTTP: 503, result: `{"code":401,"data":{"order":{"status":"completed"}}}`}, "unconfirmed", false, false},
		{"unrelated order never proves success", lookupFixture{orders: []map[string]any{{"order_id": 12, "cdk_id": 999, "status": "completed"}}}, "unconfirmed", false, false},
		{"frozen is not unused", lookupFixture{current: "frozen"}, "frozen", false, false},
	}
	for _, st := range []string{"queued", "awaiting_card", "funding_pending", "dispatching", "running", "requires_action", "pending", "plus_paid"} {
		cases = append(cases, struct {
			name        string
			f           lookupFixture
			status      string
			used, retry bool
		}{st, lookupFixture{stored: "consumed", orders: []map[string]any{fixtureOrder(12, st, "reserved")}}, "processing", false, false})
	}
	for _, st := range []string{"paid", "success", "done", "unexpected_state"} {
		cases = append(cases, struct {
			name        string
			f           lookupFixture
			status      string
			used, retry bool
		}{st, lookupFixture{orders: []map[string]any{fixtureOrder(12, st, "unused")}}, "unconfirmed", false, false})
	}
	cases = append(cases, struct {
		name        string
		f           lookupFixture
		status      string
		used, retry bool
	}{"review", lookupFixture{orders: []map[string]any{fixtureOrder(12, "review", "review")}}, "review", false, false})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupLookupFixture(t, tc.f)
			got := lookupOneCDK(context.Background(), lookupTestCode, "different-browser")
			if got.Status != tc.status || got.Used != tc.used || got.CanResubmit != tc.retry {
				t.Fatalf("got %+v; want status=%s used=%t retry=%t", got, tc.status, tc.used, tc.retry)
			}
		})
	}
}

func TestLookupSingleAndBatchAgree(t *testing.T) {
	setupLookupFixture(t, lookupFixture{stored: "consumed", orders: []map[string]any{fixtureOrder(12, "failed_precharge", "unused")}})
	router := gin.New()
	router.GET("/lookup", LookupCDKStatus)
	router.POST("/batch", LookupCDKStatusBatch)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/lookup?code="+lookupTestCode, nil))
	var one cdkLookupResult
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &one) != nil {
		t.Fatal("single request failed")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("lookup must not be cached")
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/batch", strings.NewReader(`{"codes":["`+lookupTestCode+`"]}`)))
	var many struct {
		Results []cdkLookupResult `json:"results"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &many) != nil || len(many.Results) != 1 {
		t.Fatal("batch request failed")
	}
	if one.Status != "failed" || many.Results[0].Status != one.Status || !one.CanResubmit {
		t.Fatalf("single/batch disagree: %+v %+v", one, many)
	}
}

func TestLookupCompletionMeaning(t *testing.T) {
	for _, plan := range []string{"plus", "pro_20x_renew", "credit250"} {
		r := cdkLookupResult{Plan: plan}
		applyLookupOrder(&r, map[string]any{"status": "completed", "completed_at": float64(1784397600), "updated_at": "unrelated-later-update"})
		if !r.Used || r.OrderStatus != "completed" || r.UsedAt == nil || *r.UsedAt == "unrelated-later-update" {
			t.Fatalf("invalid completion: %+v", r)
		}
		if plan == "pro_20x_renew" && !strings.Contains(r.Message, "实际续费") {
			t.Fatal("card binding is not a completed renewal payment")
		}
		if plan == "credit250" && !strings.Contains(r.Message, "点数") {
			t.Fatal("credits are not a new subscription")
		}
	}
}
