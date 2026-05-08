package store

import (
	"context"
	"encoding/json"
)

type AuditEntry struct {
	UserID  int64  // 0 for unauthenticated events
	IP      string
	Action  string
	Target  string
	Outcome string // "success" | "failure" | "denied"
	Detail  any    // serialized to JSON; may be nil
}

// WriteAudit inserts a single audit entry. Best-effort: callers should not
// fail the parent request when this returns an error, but should log it.
func (db *DB) WriteAudit(ctx context.Context, e AuditEntry) error {
	var detail any
	if e.Detail != nil {
		raw, err := json.Marshal(e.Detail)
		if err == nil {
			detail = string(raw)
		}
	}
	var userID any
	if e.UserID > 0 {
		userID = e.UserID
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO audit_log (ts, user_id, ip, action, target, outcome, detail)
		VALUES (unixepoch(), ?, ?, ?, ?, ?, ?)
	`, userID, e.IP, e.Action, nullableString(e.Target), e.Outcome, detail)
	return err
}
