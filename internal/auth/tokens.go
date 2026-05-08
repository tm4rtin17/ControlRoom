package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RefreshTokenTTL is the lifetime of a refresh token; rotation extends the
// session, but if the user is idle longer than this they need to log in again.
const RefreshTokenTTL = 7 * 24 * time.Hour

// RefreshToken is the opaque value placed in the cr_refresh cookie. It carries
// the session id and a secret half so the server can find the row and verify
// possession in O(1).
type RefreshToken struct {
	SessionID string
	Secret    string // base64url, 32 bytes of entropy
}

// String reassembles the cookie value. The two halves are joined with a dot;
// the format is intentionally not parseable without splitting on it.
func (r RefreshToken) String() string {
	return r.SessionID + "." + r.Secret
}

// SecretHash returns sha256(secret) for storage.
func (r RefreshToken) SecretHash() []byte {
	h := sha256.Sum256([]byte(r.Secret))
	return h[:]
}

// MintRefreshToken creates a fresh session id + secret pair.
func MintRefreshToken() (RefreshToken, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return RefreshToken{}, fmt.Errorf("session id: %w", err)
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return RefreshToken{}, fmt.Errorf("secret: %w", err)
	}
	return RefreshToken{
		SessionID: hex.EncodeToString(idBytes),
		Secret:    base64.RawURLEncoding.EncodeToString(secretBytes),
	}, nil
}

// MintFamilyID returns a random opaque family identifier.
func MintFamilyID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// MintCSRFToken returns 32 random bytes as base64url, suitable for the
// double-submit cookie.
func MintCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MintSetupToken returns the raw token (shown to the operator once) and its
// sha256 (stored in the DB).
func MintSetupToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	return raw, h[:], nil
}

func ParseRefreshToken(raw string) (RefreshToken, error) {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return RefreshToken{}, errors.New("malformed refresh token")
	}
	return RefreshToken{SessionID: parts[0], Secret: parts[1]}, nil
}

// ConstantTimeEqualBytes wraps subtle.ConstantTimeCompare returning a clear bool.
func ConstantTimeEqualBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// HashToken returns sha256(raw) — used for the setup token compare-by-hash.
func HashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}
