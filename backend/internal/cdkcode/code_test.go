package cdkcode

import "testing"

func TestDisplayAndParse(t *testing.T) {
	raw := "ZC-0123456789-ABCDEFGHIJ"
	for _, tc := range []struct{ plan, prefix string }{{"plus", "PLUS-"}, {"pro_5x", "Pro5X-"}, {"pro_20x", "Pro20X-"}} {
		shown := Display(raw, tc.plan)
		if shown != tc.prefix+raw[3:] {
			t.Fatalf("unexpected display for %s", tc.plan)
		}
		got, plan := Parse(shown)
		if got != raw || plan != tc.plan {
			t.Fatalf("round trip failed for %s", tc.plan)
		}
	}
	for _, plan := range []string{"", "go", "credit250", "pro_20x_renew"} {
		if Display(raw, plan) != raw {
			t.Fatalf("unexpected alias for %s", plan)
		}
	}
	if Display("GPTD-OLD-CODE", "plus") != "GPTD-OLD-CODE" {
		t.Fatal("legacy changed")
	}
}

func TestSearchAliasAndPlanMismatch(t *testing.T) {
	if p, q := Search("", "Pro5X-ABC"); p != "pro_5x" || q != "ZC-ABC" {
		t.Fatal(p, q)
	}
	if p, q := Search("", "PLUS-"); p != "plus" || q != "ZC-" {
		t.Fatal(p, q)
	}
	if p, _ := Search("plus", "Pro20X-ABC"); p == "plus" || p == "pro_20x" {
		t.Fatal("mismatched plan accepted")
	}
}
