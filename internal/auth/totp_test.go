package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTP(t *testing.T) {
	t.Parallel()
	enrol, err := GenerateTOTP("ControlRoom", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if enrol.Secret == "" {
		t.Fatal("secret should not be empty")
	}
	if !strings.HasPrefix(enrol.URI, "otpauth://totp/") {
		t.Fatalf("URI looks wrong: %s", enrol.URI)
	}
	if !strings.HasPrefix(enrol.QRDataURI, "data:image/png;base64,") {
		t.Fatalf("QR data URI looks wrong: %s", enrol.QRDataURI[:40])
	}
}

func TestGenerateTOTPRejectsEmptyAccount(t *testing.T) {
	t.Parallel()
	if _, err := GenerateTOTP("ControlRoom", ""); err == nil {
		t.Fatal("expected error for empty account")
	}
}

func TestVerifyTOTPRoundTrip(t *testing.T) {
	t.Parallel()
	enrol, err := GenerateTOTP("ControlRoom", "alice")
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(enrol.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyTOTP(enrol.Secret, code) {
		t.Fatal("fresh code should verify")
	}
}

func TestVerifyTOTPRejectsBadCode(t *testing.T) {
	t.Parallel()
	enrol, err := GenerateTOTP("ControlRoom", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if VerifyTOTP(enrol.Secret, "000000") {
		t.Fatal("000000 should not verify (extremely unlikely true positive)")
	}
}

func TestVerifyTOTPRejectsEmpty(t *testing.T) {
	t.Parallel()
	if VerifyTOTP("", "123456") {
		t.Fatal("empty secret must not verify")
	}
	if VerifyTOTP("AABB", "") {
		t.Fatal("empty code must not verify")
	}
}
