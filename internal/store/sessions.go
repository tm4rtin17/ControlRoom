package store

import (
	"context"
	"time"
)

type Session struct {
	ID           string
	UserID       int64
	RefreshHash  []byte
	FamilyID     string
	ParentID     string // empty if root of family
	IP           string
	UserAgent    string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	RevokedAt    time.Time // zero if still active
}

func (s *Session) Revoked() bool {
	return !s.RevokedAt.IsZero()
}

type CreateSessionParams struct {
	ID          string
	UserID      int64
	RefreshHash []byte
	FamilyID    string
	ParentID    string
	IP          string
	UserAgent   string
	ExpiresAt   time.Time
}

func (db *DB) CreateSession(ctx context.Context, p CreateSessionParams) error {
	now := time.Now().Unix()
	_, err := db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, refresh_hash, family_id, parent_id, ip, user_agent, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.UserID, p.RefreshHash, p.FamilyID, nullableString(p.ParentID),
		p.IP, p.UserAgent, now, p.ExpiresAt.Unix())
	return err
}

func (db *DB) SessionByID(ctx context.Context, id string) (*Session, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, user_id, refresh_hash, family_id, COALESCE(parent_id, ''), ip, COALESCE(user_agent, ''),
		       created_at, expires_at, COALESCE(revoked_at, 0)
		FROM sessions WHERE id = ?
	`, id)

	var s Session
	var createdAt, expiresAt, revokedAt int64
	if err := row.Scan(
		&s.ID, &s.UserID, &s.RefreshHash, &s.FamilyID, &s.ParentID, &s.IP, &s.UserAgent,
		&createdAt, &expiresAt, &revokedAt,
	); err != nil {
		return nil, wrapNotFound(err)
	}
	s.CreatedAt = time.Unix(createdAt, 0)
	s.ExpiresAt = time.Unix(expiresAt, 0)
	if revokedAt > 0 {
		s.RevokedAt = time.Unix(revokedAt, 0)
	}
	return &s, nil
}

func (db *DB) RevokeSession(ctx context.Context, id string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = unixepoch() WHERE id = ? AND revoked_at IS NULL
	`, id)
	return err
}

// RevokeFamily marks every still-active session in a family as revoked.
// Called on detected refresh-token reuse.
func (db *DB) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = unixepoch()
		WHERE family_id = ? AND revoked_at IS NULL
	`, familyID)
	return err
}

func (db *DB) RevokeAllForUser(ctx context.Context, userID int64) error {
	_, err := db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = unixepoch()
		WHERE user_id = ? AND revoked_at IS NULL
	`, userID)
	return err
}

// HasSessionChildren reports whether any session has the given session as its
// parent_id. Used to distinguish "this session was rotated already" (parent
// has children → replay → reuse) from "this session was simply logged out"
// (no children → just invalid).
func (db *DB) HasSessionChildren(ctx context.Context, parentID string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE parent_id = ?`,
		parentID,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
