package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtKeyFile = "jwt.key"

	// AccessTokenTTL is intentionally short so a leaked access token is
	// useful for ≤15 min; the rotating refresh provides session lifetime.
	AccessTokenTTL = 15 * time.Minute

	// SetupTokenTTL bounds the time available to complete the first-run wizard.
	SetupTokenTTL = 15 * time.Minute
)

const (
	tokenTypeAccess = "access"
	tokenTypeSetup  = "setup"
)

type Signer struct {
	key []byte
}

// LoadOrCreateSigner loads a 256-bit HS256 key from $DATA_DIR/jwt.key, or
// generates one if missing. Permissions are 0600.
func LoadOrCreateSigner(dataDir string) (*Signer, error) {
	path := filepath.Join(dataDir, jwtKeyFile)
	if data, err := os.ReadFile(path); err == nil {
		if len(data) < 32 {
			return nil, fmt.Errorf("%s too short: need >=32 bytes, got %d", path, len(data))
		}
		return &Signer{key: data}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	key := make([]byte, 64)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	return &Signer{key: key}, nil
}

// AccessClaims is the payload of access tokens.
type AccessClaims struct {
	UserID    int64  `json:"uid"`
	SessionID string `json:"sid"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Signer) IssueAccess(userID int64, sessionID, role string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		UserID:    userID,
		SessionID: sessionID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "controlroom",
			Subject:   tokenTypeAccess,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.key)
}

func (s *Signer) ParseAccess(raw string) (*AccessClaims, error) {
	out := &AccessClaims{}
	tok, err := jwt.ParseWithClaims(raw, out, s.keyFunc, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid access token")
	}
	if out.Subject != tokenTypeAccess {
		return nil, errors.New("not an access token")
	}
	return out, nil
}

// SetupClaims gates access to /api/setup/complete.
type SetupClaims struct {
	jwt.RegisteredClaims
}

func (s *Signer) IssueSetup() (string, error) {
	now := time.Now()
	claims := SetupClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "controlroom",
			Subject:   tokenTypeSetup,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(SetupTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.key)
}

func (s *Signer) ParseSetup(raw string) (*SetupClaims, error) {
	out := &SetupClaims{}
	tok, err := jwt.ParseWithClaims(raw, out, s.keyFunc, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid setup token")
	}
	if out.Subject != tokenTypeSetup {
		return nil, errors.New("not a setup token")
	}
	return out, nil
}

func (s *Signer) keyFunc(_ *jwt.Token) (any, error) { return s.key, nil }
