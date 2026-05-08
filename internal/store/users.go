package store

import (
	"context"
	"time"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash []byte
	TOTPSecret   string
	TOTPEnabled  bool
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateUserParams struct {
	Username     string
	PasswordHash []byte
	TOTPSecret   string // empty if not enrolling
	TOTPEnabled  bool
	Role         string
}

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (db *DB) CreateUser(ctx context.Context, p CreateUserParams) (*User, error) {
	now := time.Now().Unix()
	role := p.Role
	if role == "" {
		role = "admin"
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, totp_secret, totp_enabled, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, p.Username, p.PasswordHash, nullableString(p.TOTPSecret), boolToInt(p.TOTPEnabled), role, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.UserByID(ctx, id)
}

func (db *DB) UserByID(ctx context.Context, id int64) (*User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, COALESCE(totp_secret, ''), totp_enabled, role, created_at, updated_at
		FROM users WHERE id = ?
	`, id)
	return scanUser(row)
}

func (db *DB) UserByUsername(ctx context.Context, username string) (*User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, COALESCE(totp_secret, ''), totp_enabled, role, created_at, updated_at
		FROM users WHERE username = ?
	`, username)
	return scanUser(row)
}

func (db *DB) UpdatePassword(ctx context.Context, userID int64, hash []byte) error {
	_, err := db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?
	`, hash, time.Now().Unix(), userID)
	return err
}

func (db *DB) SetTOTP(ctx context.Context, userID int64, secret string, enabled bool) error {
	_, err := db.ExecContext(ctx, `
		UPDATE users SET totp_secret = ?, totp_enabled = ?, updated_at = ? WHERE id = ?
	`, nullableString(secret), boolToInt(enabled), time.Now().Unix(), userID)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var u User
	var createdAt, updatedAt int64
	var totpEnabled int
	if err := row.Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.TOTPSecret, &totpEnabled, &u.Role,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, wrapNotFound(err)
	}
	u.TOTPEnabled = totpEnabled != 0
	u.CreatedAt = time.Unix(createdAt, 0)
	u.UpdatedAt = time.Unix(updatedAt, 0)
	return &u, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
