package store

import (
	"context"
	"time"
)

type SetupToken struct {
	TokenHash []byte
	CreatedAt time.Time
	UsedAt    time.Time // zero if unused
}

func (t *SetupToken) Used() bool {
	return !t.UsedAt.IsZero()
}

// PutSetupToken replaces any existing setup token with a fresh one (id=1).
func (db *DB) PutSetupToken(ctx context.Context, hash []byte) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO setup_token (id, token_hash, created_at)
		VALUES (1, ?, unixepoch())
		ON CONFLICT(id) DO UPDATE SET
			token_hash = excluded.token_hash,
			created_at = excluded.created_at,
			used_at    = NULL
	`, hash)
	return err
}

func (db *DB) GetSetupToken(ctx context.Context) (*SetupToken, error) {
	row := db.QueryRowContext(ctx, `
		SELECT token_hash, created_at, COALESCE(used_at, 0) FROM setup_token WHERE id = 1
	`)
	var t SetupToken
	var createdAt, usedAt int64
	if err := row.Scan(&t.TokenHash, &createdAt, &usedAt); err != nil {
		return nil, wrapNotFound(err)
	}
	t.CreatedAt = time.Unix(createdAt, 0)
	if usedAt > 0 {
		t.UsedAt = time.Unix(usedAt, 0)
	}
	return &t, nil
}

func (db *DB) MarkSetupTokenUsed(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `
		UPDATE setup_token SET used_at = unixepoch() WHERE id = 1 AND used_at IS NULL
	`)
	return err
}

func (db *DB) DeleteSetupToken(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `DELETE FROM setup_token WHERE id = 1`)
	return err
}
