package auth

import (
	"context"
	"errors"
	"time"

	"github.com/tm4rtin17/controlroom/internal/store"
)

// Manager owns the rotation logic that ties refresh tokens, sessions, and
// audit together. Handlers call its methods rather than touching the DB.
type Manager struct {
	db *store.DB
}

func NewManager(db *store.DB) *Manager {
	return &Manager{db: db}
}

// IssueResult bundles everything the caller needs to set cookies.
type IssueResult struct {
	AccessToken    string
	RefreshToken   RefreshToken
	SessionID      string
	FamilyID       string
	RefreshExpires time.Time
}

// Issue starts a brand-new session family (used at login + setup completion).
func (m *Manager) Issue(ctx context.Context, signer *Signer, userID int64, role, ip, ua string) (*IssueResult, error) {
	rt, err := MintRefreshToken()
	if err != nil {
		return nil, err
	}
	familyID, err := MintFamilyID()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(RefreshTokenTTL)

	if err := m.db.CreateSession(ctx, store.CreateSessionParams{
		ID:          rt.SessionID,
		UserID:      userID,
		RefreshHash: rt.SecretHash(),
		FamilyID:    familyID,
		IP:          ip,
		UserAgent:   ua,
		ExpiresAt:   expires,
	}); err != nil {
		return nil, err
	}

	access, err := signer.IssueAccess(userID, rt.SessionID, role)
	if err != nil {
		return nil, err
	}
	return &IssueResult{
		AccessToken:    access,
		RefreshToken:   rt,
		SessionID:      rt.SessionID,
		FamilyID:       familyID,
		RefreshExpires: expires,
	}, nil
}

// ErrSessionReuse signals refresh-token replay. Caller must clear cookies and
// return 401; the family has already been revoked at this point.
var ErrSessionReuse = errors.New("session reuse detected; family revoked")

// ErrSessionInvalid covers any other refresh-time failure (expired, missing,
// secret mismatch).
var ErrSessionInvalid = errors.New("session invalid")

// Rotate validates a presented refresh token and, on success, marks the
// current session revoked and issues a new one in the same family.
//
// On reuse-of-revoked detection it revokes the entire family (defense against
// a stolen rotated token).
func (m *Manager) Rotate(ctx context.Context, signer *Signer, presented RefreshToken, ip, ua string) (*IssueResult, error) {
	sess, err := m.db.SessionByID(ctx, presented.SessionID)
	if err != nil {
		return nil, ErrSessionInvalid
	}
	if !ConstantTimeEqualBytes(sess.RefreshHash, presented.SecretHash()) {
		return nil, ErrSessionInvalid
	}
	if sess.Revoked() {
		// Distinguish replay-of-rotated-parent (reuse) from a session that was
		// simply logged out. If the revoked session already produced a child
		// (was rotated), presenting it again means an attacker is replaying
		// the old token alongside the legitimate user holding the rotated
		// child — burn the entire family.
		hasChildren, _ := m.db.HasSessionChildren(ctx, sess.ID)
		if hasChildren {
			_ = m.db.RevokeFamily(ctx, sess.FamilyID)
			return nil, ErrSessionReuse
		}
		return nil, ErrSessionInvalid
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = m.db.RevokeSession(ctx, sess.ID)
		return nil, ErrSessionInvalid
	}

	user, err := m.db.UserByID(ctx, sess.UserID)
	if err != nil {
		return nil, ErrSessionInvalid
	}

	rt, err := MintRefreshToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(RefreshTokenTTL)

	if err := m.db.CreateSession(ctx, store.CreateSessionParams{
		ID:          rt.SessionID,
		UserID:      sess.UserID,
		RefreshHash: rt.SecretHash(),
		FamilyID:    sess.FamilyID,
		ParentID:    sess.ID,
		IP:          ip,
		UserAgent:   ua,
		ExpiresAt:   expires,
	}); err != nil {
		return nil, err
	}

	if err := m.db.RevokeSession(ctx, sess.ID); err != nil {
		// Best effort — already swapped, audit will reveal anomaly.
		_ = err
	}

	access, err := signer.IssueAccess(user.ID, rt.SessionID, user.Role)
	if err != nil {
		return nil, err
	}
	return &IssueResult{
		AccessToken:    access,
		RefreshToken:   rt,
		SessionID:      rt.SessionID,
		FamilyID:       sess.FamilyID,
		RefreshExpires: expires,
	}, nil
}

// Revoke marks a single session revoked (logout).
func (m *Manager) Revoke(ctx context.Context, sessionID string) error {
	return m.db.RevokeSession(ctx, sessionID)
}
