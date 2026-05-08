package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/tm4rtin17/controlroom/internal/store"
)

func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestSigner(t *testing.T) *Signer {
	t.Helper()
	s, err := LoadOrCreateSigner(t.TempDir())
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return s
}

func newTestUser(t *testing.T, db *store.DB, name string) *store.User {
	t.Helper()
	hash, err := HashPassword("aReasonablePassword!")
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(context.Background(), store.CreateUserParams{
		Username:     name,
		PasswordHash: hash,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestSessionsIssueProducesUniqueIDs(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	signer := newTestSigner(t)
	mgr := NewManager(db)
	user := newTestUser(t, db, "alice")
	ctx := context.Background()

	r1, err := mgr.Issue(ctx, signer, user.ID, "admin", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := mgr.Issue(ctx, signer, user.ID, "admin", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if r1.SessionID == r2.SessionID {
		t.Fatal("two issues should produce different session ids")
	}
	if r1.FamilyID == r2.FamilyID {
		t.Fatal("two issues should produce different family ids")
	}
	if r1.AccessToken == r2.AccessToken {
		t.Fatal("two issues should produce different access tokens")
	}
}

func TestSessionsRotatePreservesFamily(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	signer := newTestSigner(t)
	mgr := NewManager(db)
	user := newTestUser(t, db, "bob")
	ctx := context.Background()

	first, err := mgr.Issue(ctx, signer, user.ID, "admin", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := mgr.Rotate(ctx, signer, first.RefreshToken, "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID == first.SessionID {
		t.Fatal("rotation must produce a new session id")
	}
	if second.FamilyID != first.FamilyID {
		t.Fatalf("rotation must keep family: %s vs %s", second.FamilyID, first.FamilyID)
	}
}

func TestSessionsReuseDetectionRevokesFamily(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	signer := newTestSigner(t)
	mgr := NewManager(db)
	user := newTestUser(t, db, "carol")
	ctx := context.Background()

	first, err := mgr.Issue(ctx, signer, user.ID, "admin", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := mgr.Rotate(ctx, signer, first.RefreshToken, "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}

	// Replay the original (now-revoked) refresh: must trigger reuse detection.
	if _, err := mgr.Rotate(ctx, signer, first.RefreshToken, "127.0.0.1", ""); !errors.Is(err, ErrSessionReuse) {
		t.Fatalf("expected ErrSessionReuse on replay, got %v", err)
	}

	// And the legitimate rotated child should now be invalid too — entire family revoked.
	if _, err := mgr.Rotate(ctx, signer, second.RefreshToken, "127.0.0.1", ""); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("expected ErrSessionInvalid after family revoke, got %v", err)
	}
}

func TestSessionsRotateRejectsBadSecret(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	signer := newTestSigner(t)
	mgr := NewManager(db)
	user := newTestUser(t, db, "dave")
	ctx := context.Background()

	first, err := mgr.Issue(ctx, signer, user.ID, "admin", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	tampered := RefreshToken{SessionID: first.RefreshToken.SessionID, Secret: "not-the-real-secret"}
	if _, err := mgr.Rotate(ctx, signer, tampered, "127.0.0.1", ""); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("expected ErrSessionInvalid for bad secret, got %v", err)
	}
}

func TestSessionsRotateRejectsUnknownSession(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	signer := newTestSigner(t)
	mgr := NewManager(db)
	_ = newTestUser(t, db, "erin")
	ctx := context.Background()

	bogus := RefreshToken{SessionID: "deadbeef00000000", Secret: "anything"}
	if _, err := mgr.Rotate(ctx, signer, bogus, "127.0.0.1", ""); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("expected ErrSessionInvalid, got %v", err)
	}
}

func TestSessionsRevokeLogsOut(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	signer := newTestSigner(t)
	mgr := NewManager(db)
	user := newTestUser(t, db, "frank")
	ctx := context.Background()

	first, err := mgr.Issue(ctx, signer, user.ID, "admin", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Revoke(ctx, first.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Rotate(ctx, signer, first.RefreshToken, "127.0.0.1", ""); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("revoked session should be invalid, got %v", err)
	}
}
