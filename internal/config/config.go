// Package config loads runtime configuration from environment variables.
//
// All settings have defaults; Validate enforces invariants that depend on
// combinations of values (e.g. ACME requires host + email).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type TLSMode string

const (
	TLSModeSelfSigned TLSMode = "selfsigned"
	TLSModeACME       TLSMode = "acme"
	TLSModeProxy      TLSMode = "proxy"
)

type Config struct {
	Addr         string
	DataDir      string
	TLSMode      TLSMode
	ACMEHost     string
	ACMEEmail    string
	TrustProxy   bool
	LogLevel     string
	DockerSock   string
	SessionHours int
	HostName     string
	VersionCheck bool
	DevMode      bool
}

func Load() (*Config, error) {
	c := &Config{
		Addr:         envOr("CR_ADDR", ":8443"),
		DataDir:      envOr("CR_DATA_DIR", "/var/lib/controlroom"),
		TLSMode:      TLSMode(envOr("CR_TLS_MODE", string(TLSModeSelfSigned))),
		ACMEHost:     os.Getenv("CR_ACME_HOST"),
		ACMEEmail:    os.Getenv("CR_ACME_EMAIL"),
		TrustProxy:   envBool("CR_TRUST_PROXY", false),
		LogLevel:     strings.ToLower(envOr("CR_LOG_LEVEL", "info")),
		DockerSock:   envOr("CR_DOCKER_SOCK", "/var/run/docker.sock"),
		SessionHours: envInt("CR_SESSION_HOURS", 168),
		HostName:     os.Getenv("CR_HOST_NAME"),
		VersionCheck: envBool("CR_VERSION_CHECK", false),
		DevMode:      envBool("CR_DEV", false),
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) Validate() error {
	if c.Addr == "" {
		return errors.New("CR_ADDR must be set")
	}
	if !filepath.IsAbs(c.DataDir) {
		return fmt.Errorf("CR_DATA_DIR must be absolute (got %q)", c.DataDir)
	}
	switch c.TLSMode {
	case TLSModeSelfSigned, TLSModeACME, TLSModeProxy:
	default:
		return fmt.Errorf("CR_TLS_MODE must be selfsigned|acme|proxy (got %q)", c.TLSMode)
	}
	if c.TLSMode == TLSModeACME {
		if c.ACMEHost == "" || c.ACMEEmail == "" {
			return errors.New("CR_ACME_HOST and CR_ACME_EMAIL required when CR_TLS_MODE=acme")
		}
	}
	if c.SessionHours < 1 {
		return errors.New("CR_SESSION_HOURS must be >= 1")
	}
	return nil
}

func envOr(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v, ok := os.LookupEnv(k)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(k string, def int) int {
	v, ok := os.LookupEnv(k)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
