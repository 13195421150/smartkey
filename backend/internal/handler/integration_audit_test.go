package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

func setupAuditDB(t *testing.T, handler http.HandlerFunc) {
	setupLookupFixture(t, lookupFixture{})
	for _, q := range []string{
		`CREATE TABLE card_selection_rules(id INTEGER PRIMARY KEY,sort_order INTEGER,plan_key TEXT,display_name TEXT,bin_prefix TEXT,channel TEXT,enabled INTEGER,created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE card_blocklist(card_id INTEGER, unblocked_at TEXT)`,
		`CREATE TABLE webhook_events(id INTEGER PRIMARY KEY,event_type TEXT,idem_key TEXT UNIQUE,payload TEXT,created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	} {
		if _, err := db.DB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	if err := db.SetSetting("card_api_base", srv.URL); err != nil {
		t.Fatal(err)
	}
}

func TestCardRuleSyncOnlyGPTPreservesCapacity(t *testing.T) {
	puts := 0
	setupAuditDB(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/v1/gpt-direct/card-rules" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.Method == "GET" {
			if r.URL.Query().Get("product") != "gpt" {
				t.Error("rules must be GPT scoped")
			}
			w.Write([]byte(`{"code":0,"data":{"product":"gpt","count_failures":true,"light_max_uses":5,"pro20_max_uses":3,"select_mode":"lowest_usage","max_auto_switches":2,"auto_switch_on_fail":true,"exclude_archived_cards":true}}`))
			return
		}
		if r.Method != "PUT" {
			t.Errorf("unexpected method %s", r.Method)
		}
		puts++
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		if b["product"] != "gpt" || b["light_max_uses"] != float64(5) || b["exclude_archived_cards"] != true || b["select_mode"] != "lowest_usage" {
			t.Errorf("unrelated settings changed: %#v", b)
		}
		prefs := b["select_priority"].([]any)
		if len(prefs) != 2 || prefs[1].(map[string]any)["segment_key"] != "P5556XV" {
			t.Errorf("priority order lost: %#v", prefs)
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": b})
	})
	if err := db.SetCardSelectionRules([]db.CardSelectionRule{{PlanKey: "P5378OX", Channel: "one", Enabled: true, SortOrder: 1}, {PlanKey: "P5556XV", Channel: "one", Enabled: true, SortOrder: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := SyncOwnerDirectCardRules(context.Background()); err != nil {
		t.Fatal(err)
	}
	if puts != 1 {
		t.Fatalf("must update only GPT, got %d writes", puts)
	}
}

func TestConfiguredRedeemRestrictionsCannotBeBypassed(t *testing.T) {
	setupAuditDB(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("must not call upstream") })
	if err := saveSiteRedeemPolicy(SiteRedeemPolicy{Enabled: true, NoAutoCardSwitch: true, StrictCardPreference: true}); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"no_auto_card_switch": false, "strict_card_preference": false}
	injectRedeemCardPolicy(body)
	if body["no_auto_card_switch"] != true || body["strict_card_preference"] != true {
		t.Fatal("client bypassed operator limits")
	}
	if err := saveSiteRedeemPolicy(SiteRedeemPolicy{Enabled: true, NoAutoCardSwitch: false, StrictCardPreference: false}); err != nil {
		t.Fatal(err)
	}
	injectRedeemCardPolicy(body)
	if body["no_auto_card_switch"] != true {
		t.Fatal("must respect customer's stricter no-switch request")
	}
}

func TestCardDiagnosticsAreReadOnlyAndMasked(t *testing.T) {
	setupAuditDB(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("diagnostics must be read only")
		}
		if strings.HasSuffix(r.URL.Path, "/usage") {
			w.Write([]byte(`{"code":0,"data":{"card_id":7,"card_number":"4111111111117637","card_status":"ACTIVE","light":{"used":1,"remaining":4,"cap":5},"cooling":false,"busy":false}}`))
			return
		}
		w.Write([]byte(`{"code":0,"data":{"rule":{"product":"gpt","light_max_uses":5},"candidates":[{"card_id":7,"card_number":"4111111111117637","available_usd":0.31,"light_remain":4,"skip":true,"skip_reason":"outside priority"}]}}`))
	})
	router := gin.New()
	router.GET("/pool", AdminCardPool)
	router.GET("/cards/:id/usage", AdminCardUsage)
	for _, path := range []string{"/pool?plan=plus", "/cards/7/usage"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), "4111111111117637") {
			t.Fatalf("unsafe/failed response: %d %s", w.Code, w.Body)
		}
	}
}

func TestWebhookStableIDsLifecycleAndStorageFailure(t *testing.T) {
	setupAuditDB(t, func(w http.ResponseWriter, r *http.Request) { t.Error("webhook must not issue orders") })
	db.SetSetting("webhook_secret", "fixture-hook-secret")
	router := gin.New()
	router.POST("/hook", CardPlatformWebhook)
	send := func(raw string, signed bool) int {
		req := httptest.NewRequest("POST", "/hook", strings.NewReader(raw))
		if signed {
			mac := hmac.New(sha256.New, []byte("fixture-hook-secret"))
			mac.Write([]byte(raw))
			req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	released := `{"type":"cdk.released","event_id":"cdk.released:7:12","cdk_id":7,"cdk_status":"unused"}`
	if send(released, false) != 401 || send("invalid-json", true) != 400 {
		t.Fatal("invalid webhook accepted")
	}
	db.UpdateCardplatformCDKStatus(7, "reserved")
	if send(released, true) != 200 || db.GetCardplatformCDKStatus(7) != "unused" {
		t.Fatal("release lifecycle not applied")
	}
	db.UpdateCardplatformCDKStatus(7, "reserved")
	if send(released, true) != 200 || db.GetCardplatformCDKStatus(7) != "reserved" {
		t.Fatal("duplicate replay changed state")
	}
	if send(`{"type":"cdk.consumed","event_id":"consumed:7","cdk_id":7}`, true) != 200 {
		t.Fatal("consumed failed")
	}
	send(`{"type":"cdk.released","event_id":"older-release:7","cdk_id":7}`, true)
	if db.GetCardplatformCDKStatus(7) != "consumed" {
		t.Fatal("late failure regressed completed CDK")
	}
	db.DB.Exec("DROP TABLE webhook_events")
	if send(`{"type":"cdk.frozen","event_id":"freeze:8","cdk_id":8}`, true) != 503 {
		t.Fatal("storage failure must request webhook retry")
	}
}

func TestFailedIssuanceNeverReturnsHistoricalCodes(t *testing.T) {
	for _, purchasable := range []bool{true, false} {
		t.Run(map[bool]string{true: "uncertain", false: "unavailable"}[purchasable], func(t *testing.T) {
			issues := 0
			setupAuditDB(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/openapi/v1/gpt-direct/plans":
					json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"plans": map[string]any{"plus": map[string]any{"enabled": true}}, "registry": []map[string]any{{"key": "plus", "product": "gpt", "purchasable": purchasable}}}})
				case r.URL.Path == "/openapi/v1/gpt-direct/cdks" && r.Method == "POST":
					issues++
					if r.Header.Get("Idempotency-Key") != "fixture-issue" {
						t.Error("idempotency key lost")
					}
					w.WriteHeader(503)
					w.Write([]byte(`{"code":503,"msg":"fixture unavailable"}`))
				default:
					t.Errorf("must not recover unrelated historical codes: %s %s", r.Method, r.URL)
					w.WriteHeader(404)
				}
			})
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("POST", "/issue", bytes.NewBufferString(`{"plan":"plus","count":1,"funding_confirmed":true}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Request.Header.Set("Idempotency-Key", "fixture-issue")
			CardPlatformIssueCDKs(ctx)
			if w.Code < 400 || strings.Contains(w.Body.String(), `"issued"`) {
				t.Fatalf("false success: %s", w.Body)
			}
			if !purchasable && issues != 0 {
				t.Fatal("unavailable plan was purchased")
			}
		})
	}
}
