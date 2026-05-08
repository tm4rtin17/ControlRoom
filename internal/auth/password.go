// Package auth implements the security primitives shared by HTTP handlers:
// password hashing, TOTP, JWTs, opaque session tokens, and rotation logic.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is tuned for ~250-400ms on a Raspberry Pi 5; raise as the
	// hardware floor improves. Bcrypt's 72-byte input limit is enforced below.
	BcryptCost = 12

	MinPasswordLength = 12
	MaxPasswordLength = 72
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 12 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 72 characters")
)

func HashPassword(plain string) ([]byte, error) {
	if len(plain) < MinPasswordLength {
		return nil, ErrPasswordTooShort
	}
	if len(plain) > MaxPasswordLength {
		return nil, ErrPasswordTooLong
	}
	return bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
}

// VerifyPassword runs in constant time relative to the hash; safe under timing
// observation. Returns false on any error (mismatch, malformed hash, …).
func VerifyPassword(hash []byte, plain string) bool {
	if len(plain) == 0 || len(plain) > MaxPasswordLength {
		return false
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(plain)) == nil
}
