package db

import (
	"database/sql"
	"testing"
)

func TestPruneCardProductsExceptKeepsPresentAndIgnoresEmpty(t *testing.T) {
	old := DB
	c, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	DB = c
	t.Cleanup(func() { _ = c.Close(); DB = old })
	c.SetMaxOpenConns(1)
	if _, err := c.Exec(`CREATE TABLE card_product_cache(product_code TEXT PRIMARY KEY); INSERT INTO card_product_cache VALUES('still-listed'),('delisted')`); err != nil {
		t.Fatal(err)
	}
	if removed, err := PruneCardProductsExcept(map[string]bool{}); err != nil || removed != 0 {
		t.Fatalf("empty API response must not clear cache: removed=%d err=%v", removed, err)
	}
	if removed, err := PruneCardProductsExcept(map[string]bool{"still-listed": true}); err != nil || removed != 1 {
		t.Fatalf("expected one delisted product to be removed: removed=%d err=%v", removed, err)
	}
	var count int
	if err := c.QueryRow(`SELECT count(*) FROM card_product_cache WHERE product_code='still-listed'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("listed product missing: count=%d err=%v", count, err)
	}
	if err := c.QueryRow(`SELECT count(*) FROM card_product_cache WHERE product_code='delisted'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("delisted product remains: count=%d err=%v", count, err)
	}
}
