package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name:   "defaults are valid",
			mutate: func(c *Config) {},
		},
		{
			name:    "empty addr",
			mutate:  func(c *Config) { c.Addr = "" },
			wantErr: "CR_ADDR",
		},
		{
			name:    "relative data dir",
			mutate:  func(c *Config) { c.DataDir = "relative/path" },
			wantErr: "absolute",
		},
		{
			name:    "unknown tls mode",
			mutate:  func(c *Config) { c.TLSMode = "weird" },
			wantErr: "CR_TLS_MODE",
		},
		{
			name:    "acme without host",
			mutate:  func(c *Config) { c.TLSMode = TLSModeACME; c.ACMEEmail = "x@y.z" },
			wantErr: "CR_ACME_HOST",
		},
		{
			name:    "acme without email",
			mutate:  func(c *Config) { c.TLSMode = TLSModeACME; c.ACMEHost = "x.example.com" },
			wantErr: "CR_ACME_HOST",
		},
		{
			name: "acme with both",
			mutate: func(c *Config) {
				c.TLSMode = TLSModeACME
				c.ACMEHost = "x.example.com"
				c.ACMEEmail = "x@y.z"
			},
		},
		{
			name:    "session hours zero",
			mutate:  func(c *Config) { c.SessionHours = 0 },
			wantErr: "CR_SESSION_HOURS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Addr:         ":8443",
				DataDir:      "/var/lib/controlroom",
				TLSMode:      TLSModeSelfSigned,
				LogLevel:     "info",
				DockerSock:   "/var/run/docker.sock",
				SessionHours: 168,
			}
			tt.mutate(c)
			err := c.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}
