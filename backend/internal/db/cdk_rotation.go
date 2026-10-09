package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/tuzi/cdk-recharge-system/internal/cdkcode"
)

var ErrCDKRotationConflict = errors.New("卡密已变更，请刷新列表后重试")
var ErrCDKSessionRevoked = errors.New("兑换会话已失效，请重新输入有效卡密")

type SiteCDK struct {
	Code       string `json:"code"`
	Generation int64  `json:"generation"`
	RequestID  string `json:"-"`
}

// Original upstream codes and all task/session history remain in their original tables.
func InitCDKRotationSchema() error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS cdk_site_codes(canonical_code TEXT PRIMARY KEY, site_code TEXT NOT NULL UNIQUE COLLATE NOCASE, generation INTEGER NOT NULL, request_id TEXT NOT NULL, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS cdk_site_retired(code_hash TEXT PRIMARY KEY, canonical_code TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS cdk_site_tokens(token_hash TEXT PRIMARY KEY, canonical_code TEXT NOT NULL, token TEXT NOT NULL, device TEXT NOT NULL DEFAULT '', generation INTEGER NOT NULL, submitted INTEGER NOT NULL DEFAULT 0)`,
		`CREATE INDEX IF NOT EXISTS idx_site_token_canonical ON cdk_site_tokens(canonical_code)`,
	} {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(h[:])
}

func SiteCDKFor(code, plan string) (SiteCDK, error) {
	var v SiteCDK
	err := DB.QueryRow(`SELECT site_code,generation,request_id FROM cdk_site_codes WHERE canonical_code=?`, normalizeCDKCode(code)).Scan(&v.Code, &v.Generation, &v.RequestID)
	if errors.Is(err, sql.ErrNoRows) {
		return SiteCDK{Code: cdkcode.Display(code, plan)}, nil
	}
	return v, err
}

// Caller holds the same per-code lock used by preview and redemption.
func RotateSiteCDK(code, plan, requestID string, expected int64) (SiteCDK, error) {
	code = normalizeCDKCode(code)
	tx, err := DB.Begin()
	if err != nil {
		return SiteCDK{}, err
	}
	defer tx.Rollback()
	old := SiteCDK{Code: cdkcode.Display(code, plan)}
	err = tx.QueryRow(`SELECT site_code,generation,request_id FROM cdk_site_codes WHERE canonical_code=?`, code).Scan(&old.Code, &old.Generation, &old.RequestID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SiteCDK{}, err
	}
	if old.RequestID == requestID && old.Generation > 0 {
		return old, nil
	}
	if old.Generation != expected {
		return SiteCDK{}, ErrCDKRotationConflict
	}
	var bytes [16]byte
	if _, err = rand.Read(bytes[:]); err != nil {
		return SiteCDK{}, err
	}
	prefix := cdkcode.Prefix(plan)
	if prefix == "" {
		prefix = "CDK-"
	}
	suffix := strings.ToUpper(hex.EncodeToString(bytes[:]))
	next := SiteCDK{Code: prefix + suffix[:16] + "-" + suffix[16:], Generation: old.Generation + 1, RequestID: requestID}
	for _, retired := range []string{old.Code, code, cdkcode.Display(code, plan)} {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO cdk_site_retired(code_hash,canonical_code) VALUES(?,?)`, HashCDKCode(retired), code); err != nil {
			return SiteCDK{}, err
		}
	}
	_, err = tx.Exec(`INSERT INTO cdk_site_codes(canonical_code,site_code,generation,request_id) VALUES(?,?,?,?) ON CONFLICT(canonical_code) DO UPDATE SET site_code=excluded.site_code,generation=excluded.generation,request_id=excluded.request_id,updated_at=CURRENT_TIMESTAMP`, code, next.Code, next.Generation, requestID)
	if err != nil {
		return SiteCDK{}, err
	}
	return next, tx.Commit()
}

// Legacy latest bindings remain valid until the first rotation. All newly issued tokens
// are tracked, including tokens superseded by previews in another browser.
func ValidateSiteCDKToken(token string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", ErrCDKSessionRevoked
	}
	var code string
	var epoch, current int64
	err := DB.QueryRow(`SELECT t.canonical_code,t.generation,COALESCE(s.generation,0) FROM cdk_site_tokens t LEFT JOIN cdk_site_codes s ON s.canonical_code=t.canonical_code WHERE t.token_hash=?`, tokenHash(token)).Scan(&code, &epoch, &current)
	if errors.Is(err, sql.ErrNoRows) {
		err = DB.QueryRow(`SELECT b.cdk_code,COALESCE(s.generation,0) FROM cdk_session_bindings b LEFT JOIN cdk_site_codes s ON s.canonical_code=b.cdk_code WHERE b.redemption_token=? LIMIT 1`, strings.TrimSpace(token)).Scan(&code, &current)
		epoch = 0
	}
	if errors.Is(err, sql.ErrNoRows) || (err == nil && epoch != current) {
		return "", ErrCDKSessionRevoked
	}
	return code, err
}

func MarkSiteCDKSubmitted(code, token string) error {
	upstream, err := UpstreamCDKToken(token)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`INSERT INTO cdk_site_tokens(token_hash,canonical_code,token,generation,submitted) VALUES(?,?,?,COALESCE((SELECT generation FROM cdk_site_codes WHERE canonical_code=?),0),1) ON CONFLICT(token_hash) DO UPDATE SET submitted=1`, tokenHash(token), normalizeCDKCode(code), upstream, normalizeCDKCode(code))
	return err
}

type SiteCDKToken struct {
	Token  string
	Device string
}

func PendingSiteCDKTokens(code string) ([]SiteCDKToken, error) {
	rows, err := DB.Query(`SELECT DISTINCT token,device FROM cdk_site_tokens WHERE canonical_code=? AND generation=COALESCE((SELECT generation FROM cdk_site_codes WHERE canonical_code=?),0) AND submitted=1`, normalizeCDKCode(code), normalizeCDKCode(code))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := []SiteCDKToken{}
	for rows.Next() {
		var token SiteCDKToken
		if err := rows.Scan(&token.Token, &token.Device); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

// Only a confirmed terminal result may clear a submitted token's guard.
func ClearSiteCDKSubmitted(token string) error {
	_, err := DB.Exec(`UPDATE cdk_site_tokens SET submitted=0 WHERE token_hash=?`, tokenHash(token))
	return err
}

func RegisterSiteCDKToken(tx *sql.Tx, code, token, upstreamToken, device string) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO cdk_site_tokens(token_hash,canonical_code,token,device,generation) VALUES(?,?,?,?,COALESCE((SELECT generation FROM cdk_site_codes WHERE canonical_code=?),0))`, tokenHash(token), code, strings.TrimSpace(upstreamToken), device, code)
	return err
}

func CanonicalCDKForID(id int64) (string, string, error) {
	var code, plan string
	err := DB.QueryRow(`SELECT code,COALESCE(plan,'') FROM cardplatform_cdk_codes WHERE upstream_id=? ORDER BY created_at DESC LIMIT 1`, id).Scan(&code, &plan)
	if err == nil && code == "" {
		err = fmt.Errorf("empty canonical code")
	}
	return code, plan, err
}

// Each site preview gets its own opaque token, even if upstream reuses a session.
func NewSiteCDKToken() (string, error) {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "scdk_" + hex.EncodeToString(buf[:]), nil
}

func UpstreamCDKToken(siteToken string) (string, error) {
	var upstream string
	err := DB.QueryRow(`SELECT token FROM cdk_site_tokens WHERE token_hash=?`, tokenHash(siteToken)).Scan(&upstream)
	if errors.Is(err, sql.ErrNoRows) {
		return siteToken, nil
	} // Existing bindings from before this release.
	return upstream, err
}

func ClearSiteCDKSubmittedUpstream(code, upstreamToken string) error {
	_, err := DB.Exec(`UPDATE cdk_site_tokens SET submitted=0 WHERE canonical_code=? AND token=?`, normalizeCDKCode(code), upstreamToken)
	return err
}
