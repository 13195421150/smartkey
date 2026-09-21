package cdkcode

import "strings"

// Only these three plans have site-specific aliases. The upstream secret is unchanged.
func Prefix(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "plus":
		return "PLUS-"
	case "pro_5x":
		return "Pro5X-"
	case "pro_20x":
		return "Pro20X-"
	}
	return ""
}

func Display(code, plan string) string {
	code = strings.TrimSpace(code)
	if prefix := Prefix(plan); prefix != "" && strings.HasPrefix(strings.ToUpper(code), "ZC-") {
		return prefix + code[3:]
	}
	return code
}

// Parse identifies an alias; callers must verify its plan before accepting it.
func Parse(code string) (canonical, plan string) {
	code = strings.TrimSpace(code)
	upper := strings.ToUpper(code)
	for _, plan := range []string{"plus", "pro_5x", "pro_20x"} {
		prefix := Prefix(plan)
		if strings.HasPrefix(upper, strings.ToUpper(prefix)) {
			return "ZC-" + upper[len(prefix):], plan
		}
	}
	return code, ""
}

// Search accepts complete aliases, partial aliases and a plan prefix on its own.
func Search(plan, query string) (string, string) {
	canonical, aliasPlan := Parse(query)
	if aliasPlan == "" {
		return plan, query
	}
	if plan != "" && plan != aliasPlan {
		return "__mismatched_cdk_prefix__", canonical
	}
	return aliasPlan, canonical
}
