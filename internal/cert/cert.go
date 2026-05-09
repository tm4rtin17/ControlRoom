// Package cert provides TLS configuration for the server.
//
// v0.1 implements the self-signed mode: a long-lived ECDSA P-256 cert is
// generated on first boot under $DATA_DIR/tls/ and reloaded thereafter.
// ACME and reverse-proxy modes are stubbed and will be filled in during M9.
package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/tm4rtin17/controlroom/internal/config"
)

const (
	certDir      = "tls"
	certFile     = "controlroom.crt"
	keyFile      = "controlroom.key"
	certValidity = 10 * 365 * 24 * time.Hour
)

// LoadOrGenerate returns a TLS config ready for net.Listen.
//
// For ACME mode it also returns an http.Handler that the caller MUST serve on
// :80 — that endpoint serves the HTTP-01 challenge and redirects everything
// else to HTTPS. For self-signed and proxy modes the handler is nil.
func LoadOrGenerate(cfg *config.Config) (*tls.Config, http.Handler, error) {
	switch cfg.TLSMode {
	case config.TLSModeSelfSigned:
		t, err := loadOrGenerateSelfSigned(cfg.DataDir)
		return t, nil, err
	case config.TLSModeACME:
		return loadACME(cfg)
	case config.TLSModeProxy:
		return nil, nil, errors.New("proxy mode means TLS is terminated upstream; bind plain HTTP and put ControlRoom behind your reverse proxy")
	default:
		return nil, nil, fmt.Errorf("unknown tls mode %q", cfg.TLSMode)
	}
}

// loadACME wires golang.org/x/crypto/acme/autocert against $DATA_DIR/acme.
//
// Prerequisites at the host level:
//   - The host must be reachable on :80 from the public internet for the
//     HTTP-01 challenge.
//   - The configured CR_ACME_HOST must resolve to this host.
func loadACME(cfg *config.Config) (*tls.Config, http.Handler, error) {
	if cfg.ACMEHost == "" || cfg.ACMEEmail == "" {
		return nil, nil, errors.New("CR_ACME_HOST and CR_ACME_EMAIL are required when CR_TLS_MODE=acme")
	}
	cacheDir := filepath.Join(cfg.DataDir, "acme")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("acme cache dir: %w", err)
	}
	m := &autocert.Manager{
		Cache:      autocert.DirCache(cacheDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.ACMEHost),
		Email:      cfg.ACMEEmail,
	}
	tlsCfg := m.TLSConfig()
	// Apply the same hardening the self-signed path uses.
	tlsCfg.MinVersion = tls.VersionTLS12
	tlsCfg.NextProtos = append([]string{"h2", "http/1.1"}, tlsCfg.NextProtos...)
	return tlsCfg, m.HTTPHandler(nil), nil
}

func loadOrGenerateSelfSigned(dataDir string) (*tls.Config, error) {
	dir := filepath.Join(dataDir, certDir)
	certPath := filepath.Join(dir, certFile)
	keyPath := filepath.Join(dir, keyFile)

	if fileExists(certPath) && fileExists(keyPath) {
		c, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("loading existing self-signed cert: %w", err)
		}
		return baseTLSConfig(c), nil
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating tls dir: %w", err)
	}

	c, err := generateSelfSigned(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return baseTLSConfig(c), nil
}

func generateSelfSigned(certPath, keyPath string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generating key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("serial: %w", err)
	}

	hostname, _ := os.Hostname()
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"ControlRoom"},
			CommonName:   firstNonEmpty(hostname, "controlroom"),
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(certValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              dedupeStrings([]string{"localhost", hostname}),
		IPAddresses:           localIPs(),
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("creating cert: %w", err)
	}

	if err := writePEM(certPath, "CERTIFICATE", der, 0o644); err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshalling key: %w", err)
	}
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return tls.Certificate{}, err
	}

	return tls.LoadX509KeyPair(certPath, keyPath)
}

// baseTLSConfig returns a hardened TLS config (TLS 1.2+, modern suites; TLS 1.3
// uses its own fixed suite list and ignores CipherSuites).
func baseTLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		NextProtos:       []string{"http/1.1"},
	}
}

func writePEM(path, blockType string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: data})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func localIPs() []net.IP {
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if v4 := ipnet.IP.To4(); v4 != nil {
			ips = append(ips, v4)
		}
	}
	return ips
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
