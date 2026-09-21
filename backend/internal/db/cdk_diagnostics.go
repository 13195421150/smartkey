package db

import "database/sql"

type CDKAttemptDiagnostic struct {
	RequestID  string `json:"-"`
	Phase      string `json:"phase"`
	ErrorCode  string `json:"error_code,omitempty"`
	Message    string `json:"message"`
	Uncertain  bool   `json:"uncertain"`
	StartedAt  int64  `json:"-"`
	OccurredAt string `json:"occurred_at"`
}

func SaveCDKAttempt(code string, d CDKAttemptDiagnostic) error {
	_, err := DB.Exec(`INSERT INTO cdk_attempt_diagnostics(cdk_code,phase,request_id,error_code,message,uncertain,started_at,occurred_at)
	 VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(cdk_code) DO UPDATE SET phase=excluded.phase,request_id=excluded.request_id,error_code=excluded.error_code,
	 message=excluded.message,uncertain=excluded.uncertain,started_at=excluded.started_at,occurred_at=excluded.occurred_at
	 WHERE excluded.started_at>=cdk_attempt_diagnostics.started_at`, normalizeCDKCode(code), d.Phase, d.RequestID, d.ErrorCode, d.Message, d.Uncertain, d.StartedAt, d.OccurredAt)
	return err
}

func GetCDKAttempt(code string) (*CDKAttemptDiagnostic, error) {
	var d CDKAttemptDiagnostic
	err := DB.QueryRow(`SELECT phase,request_id,error_code,message,uncertain,started_at,occurred_at FROM cdk_attempt_diagnostics WHERE cdk_code=?`, normalizeCDKCode(code)).Scan(&d.Phase, &d.RequestID, &d.ErrorCode, &d.Message, &d.Uncertain, &d.StartedAt, &d.OccurredAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if d.Message == "" {
		return nil, nil
	}
	return &d, nil
}
