package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("aReasonablePassword!")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "aReasonablePassword!") {
		t.Fatal("verify should succeed on round-trip")
	}
	if VerifyPassword(hash, "different password") {
		t.Fatal("verify should fail with wrong password")
	}
}

func TestHashPasswordRejectsTooShort(t *testing.T) {
	t.Parallel()
	_, err := HashPassword("short")
	if err == nil {
		t.Fatal("expected error for too-short password")
	}
	if !strings.Contains(err.Error(), "12") {
		t.Fatalf("expected length-12 error, got %q", err.Error())
	}
}

func TestHashPasswordRejectsTooLong(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 73)
	_, err := HashPassword(long)
	if err == nil {
		t.Fatal("expected error for too-long password")
	}
	if !strings.Contains(err.Error(), "72") {
		t.Fatalf("expected length-72 error, got %q", err.Error())
	}
}

func TestVerifyPasswordRejectsEmpty(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("aReasonablePassword!")
	if err != nil {
		t.Fatal(err)
	}
	if VerifyPassword(hash, "") {
		t.Fatal("empty password must never verify")
	}
}

func TestVerifyPasswordRejectsOversized(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("aReasonablePassword!")
	if err != nil {
		t.Fatal(err)
	}
	huge := strings.Repeat("x", 1024)
	if VerifyPassword(hash, huge) {
		t.Fatal("oversized password must not verify")
	}
}
