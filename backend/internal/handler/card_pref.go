package handler

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tuzi/cdk-recharge-system/internal/cardplatform"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

func hasEnabledSelectionRules() bool {
	rules, err := db.GetCardSelectionRules()
	if err != nil {
		return false
	}
	for _, r := range rules {
		if r.Enabled && strings.TrimSpace(r.PlanKey) != "" {
			return true
		}
	}
	return false
}

func cardProductUsable(code string, products []db.CardProductCache) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	if len(products) == 0 {
		return true
	}
	for _, p := range products {
		if !strings.EqualFold(p.ProductCode, code) {
			continue
		}
		return p.Enabled && strings.TrimSpace(p.SuspendedAt) == ""
	}
	return false
}

func resolveIssuerForRule(r db.CardSelectionRule, products []db.CardProductCache) string {
	if iss := cardplatform.CanonicalCardIssuer(r.Channel); iss != "" {
		return iss
	}
	for _, p := range products {
		if strings.EqualFold(p.ProductCode, strings.TrimSpace(r.PlanKey)) {
			if iss := cardplatform.CanonicalCardIssuer(p.Issuer); iss != "" {
				return iss
			}
		}
	}
	return "one"
}

// buildSelectPriority 把本站选卡规则转成卡台 select_priority；已下线/未启用的跳过。
func buildSelectPriority(rules []db.CardSelectionRule, products []db.CardProductCache) []cardplatform.DirectCardSelectPref {
	out := make([]cardplatform.DirectCardSelectPref, 0, len(rules))
	seen := map[string]bool{}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		key := strings.TrimSpace(r.PlanKey)
		if key == "" || !cardProductUsable(key, products) {
			continue
		}
		iss := resolveIssuerForRule(r, products)
		dup := iss + "|product|" + key
		if seen[dup] {
			continue
		}
		seen[dup] = true
		out = append(out, cardplatform.DirectCardSelectPref{
			Issuer: iss, SegmentType: "product", SegmentKey: key,
		})
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func firstUsableCardPref(policy SiteRedeemPolicy, rules []db.CardSelectionRule, products []db.CardProductCache) (issuer, segmentType, segmentKey string) {
	if code := strings.TrimSpace(policy.ProductCode); policy.Enabled && code != "" {
		if cardProductUsable(code, products) {
			iss := cardplatform.CanonicalCardIssuer(policy.Issuer)
			if iss == "" {
				for _, p := range products {
					if strings.EqualFold(p.ProductCode, code) {
						iss = cardplatform.CanonicalCardIssuer(p.Issuer)
						break
					}
				}
			}
			if iss == "" {
				iss = "one"
			}
			return iss, "product", code
		}
	}
	prefs := buildSelectPriority(rules, products)
	if len(prefs) == 0 {
		return "", "", ""
	}
	return prefs[0].Issuer, prefs[0].SegmentType, prefs[0].SegmentKey
}

// resolveIssueCardPref 发码/换码用的第一条可用偏好。
// 不再要求「本站策略」开关：选卡配置本身就该生效。
func resolveIssueCardPref(policy SiteRedeemPolicy) (issuer, segmentType, segmentKey string) {
	rules, _ := db.GetCardSelectionRules()
	products, _ := db.GetCardProducts()
	return firstUsableCardPref(policy, rules, products)
}

func issuePrefFromSite() (cardplatform.IssueCardPref, bool) {
	issuer, segType, segKey := resolveIssueCardPref(loadSiteRedeemPolicy())
	if segKey == "" && issuer == "" {
		return cardplatform.IssueCardPref{}, false
	}
	return cardplatform.IssueCardPref{Issuer: issuer, SegmentType: segType, SegmentKey: segKey}, true
}

// injectRedeemCardPolicy 兑换请求写入卡台：有选卡配置就 strict，避免被 537872/星链盖过。
func injectRedeemCardPolicy(body map[string]any) {
	if body == nil || db.DB == nil {
		return
	}
	policy := loadSiteRedeemPolicy()
	hasRules := hasEnabledSelectionRules()
	if policy.Enabled {
		body["no_auto_card_switch"] = policy.NoAutoCardSwitch || body["no_auto_card_switch"] == true
	}
	if policy.Enabled || hasRules {
		if policy.Enabled {
			body["strict_card_preference"] = policy.StrictCardPreference || body["strict_card_preference"] == true
		} else {
			body["strict_card_preference"] = true
		}
	}
}

// SyncOwnerDirectCardRules 把本站选卡优先级推到卡台所有者账户。
// 兑换走所有者名下的卡，只改 CDK 本地配置、不写卡台规则，卡台仍会走自己的 537872 级联。
func SyncOwnerDirectCardRules(ctx context.Context) error {
	cfg := cardplatform.LoadConfig()
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("card_api_key not configured; rules were not synchronized")
	}
	rules, err := db.GetCardSelectionRules()
	if err != nil {
		return err
	}
	products, _ := db.GetCardProducts()
	prefs := buildSelectPriority(rules, products)
	policy := loadSiteRedeemPolicy()
	if policy.Enabled && strings.TrimSpace(policy.ProductCode) != "" {
		iss, typ, key := firstUsableCardPref(policy, rules, products)
		if key == strings.TrimSpace(policy.ProductCode) {
			first := cardplatform.DirectCardSelectPref{Issuer: iss, SegmentType: typ, SegmentKey: key}
			ordered := []cardplatform.DirectCardSelectPref{first}
			for _, p := range prefs {
				if p != first {
					ordered = append(ordered, p)
				}
			}
			prefs = ordered
		}
	}
	cli := cardplatform.New(cfg)
	existing, err := cli.GetCardRules(ctx, "gpt")
	if err != nil {
		return err
	}
	// Only GPT is managed by this portal. Preserve the existing capacity and safety rules.
	var current *cardplatform.DirectCardRule
	for i := range existing {
		if existing[i].Product == "gpt" {
			current = &existing[i]
			break
		}
	}
	if current == nil {
		return fmt.Errorf("card platform did not return GPT rules; nothing synchronized")
	}
	current.SelectPriority = prefs
	current.StrictSelect = len(prefs) > 0
	if policy.Enabled {
		current.AutoSwitchOnFail = !policy.NoAutoCardSwitch
		current.StrictSelect = len(prefs) > 0 && policy.StrictCardPreference
	}
	if _, err := cli.PutCardRule(ctx, *current); err != nil {
		return err
	}
	log.Printf("[card-rules-sync] product=gpt priority=%d strict=%v", len(prefs), current.StrictSelect)
	return nil
}
