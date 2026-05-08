package auth

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Enrollment is everything the SPA needs to render a "scan this QR" step
// without doing crypto in JavaScript.
type Enrollment struct {
	Secret    string // base32-encoded shared secret
	URI       string // otpauth://totp/...
	QRDataURI string // data:image/png;base64,…
}

// GenerateTOTP creates a new secret + QR for an account. Caller stores the
// secret only after the user proves they can read codes from it (verify step).
func GenerateTOTP(issuer, account string) (*Enrollment, error) {
	if issuer == "" {
		issuer = "ControlRoom"
	}
	if account == "" {
		return nil, fmt.Errorf("account required")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Period:      30,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1, // widest authenticator-app support
	})
	if err != nil {
		return nil, fmt.Errorf("totp generate: %w", err)
	}

	img, err := key.Image(256, 256)
	if err != nil {
		return nil, fmt.Errorf("totp qr image: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}

	return &Enrollment{
		Secret:    key.Secret(),
		URI:       key.URL(),
		QRDataURI: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

// VerifyTOTP returns true if code is valid for the given secret within the
// default ±1-step window. The pquerna lib's Validate uses constant-time compare.
func VerifyTOTP(secret, code string) bool {
	if secret == "" || len(code) == 0 {
		return false
	}
	return totp.Validate(code, secret)
}
