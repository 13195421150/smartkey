package db

import (
	"database/sql"
	"errors"
	"testing"
)

func TestAliasesShareOriginalRecordAndRequireMatchingPlan(t *testing.T) {
	old := DB
	c, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	DB = c
	t.Cleanup(func() { c.Close(); DB = old })
	c.SetMaxOpenConns(1)
	_, err = c.Exec(`CREATE TABLE cardplatform_cdk_codes (upstream_id INTEGER,code TEXT UNIQUE,code_prefix TEXT,plan TEXT,fee_amount_minor INTEGER,status TEXT,created_at TEXT);CREATE TABLE cardplatform_cdk_notes(upstream_id INTEGER,note TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	raw := "ZC-0123456789-ABCDEFGHIJ"
	if err = SaveCardplatformCDKCode(12, raw, "ZC-0123456789", "plus", 15); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{raw, "PLUS-0123456789-ABCDEFGHIJ", "plus-0123456789-abcdefghij"} {
		if got, err := ResolveCDKCode(input); err != nil || got != raw {
			t.Fatal(got, err)
		}
	}
	for _, input := range []string{"Pro20X-0123456789-ABCDEFGHIJ", "Pro5X-0123456789-ABCDEFGHIJ", "PLUS-MISSING-CODE", "PLUS-"} {
		if _, err := ResolveCDKCode(input); !errors.Is(err, ErrCDKAlias) {
			t.Fatalf("invalid alias accepted: %v", err)
		}
	}
	c.Exec("UPDATE cardplatform_cdk_codes SET status='consumed'")
	if err = SaveCardplatformCDKCode(12, "PLUS-0123456789-ABCDEFGHIJ", "PLUS-0123456789", "plus", 15); err != nil {
		t.Fatal(err)
	}
	var count int
	c.QueryRow("SELECT COUNT(*) FROM cardplatform_cdk_codes").Scan(&count)
	if count != 1 {
		t.Fatal("alias stored as second code")
	}
	var status string
	c.QueryRow("SELECT status FROM cardplatform_cdk_codes").Scan(&status)
	if status != "consumed" {
		t.Fatal("cache import revived consumed code")
	}
	rows, total, err := ListCardplatformStoredCDKCodesPage("", "PLUS-", "", 1, 20)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Code != raw {
		t.Fatal(total, err)
	}
	if _, total, err = ListCardplatformStoredCDKCodesPage("", "Pro20X-0123456789-ABCDEFGHIJ", "", 1, 20); err != nil || total != 0 {
		t.Fatal(total, err)
	}
	if err = SaveCardplatformCDKCode(12, "Pro20X-0123456789-ABCDEFGHIJ", "", "plus", 15); !errors.Is(err, ErrCDKAlias) {
		t.Fatal("mismatched import accepted")
	}
}
