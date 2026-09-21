package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tuzi/cdk-recharge-system/internal/cdkcode"
)

var ErrCDKAlias = errors.New("卡密无效或套餐前缀不匹配")

// ResolveCDKCode keeps bindings, diagnostics and one-time use on the original code.
// An alias is accepted only for a stored full code with the corresponding plan.
func ResolveCDKCode(input string) (string, error) {
	canonical, expectedPlan := cdkcode.Parse(input)
	if expectedPlan == "" {
		return canonical, nil
	}
	if DB == nil {
		return "", fmt.Errorf("db not init")
	}
	var code, plan string
	err := DB.QueryRow(`SELECT code, COALESCE(plan,'') FROM cardplatform_cdk_codes
		WHERE upper(trim(code)) = ? ORDER BY created_at DESC LIMIT 1`, canonical).Scan(&code, &plan)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !strings.EqualFold(plan, expectedPlan)) {
		return "", ErrCDKAlias
	}
	if err != nil {
		return "", err
	}
	return code, nil
}
