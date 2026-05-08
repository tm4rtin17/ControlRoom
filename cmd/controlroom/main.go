// Command controlroom is the single-binary backend for the ControlRoom homelab UI.
//
// Boot sequence:
//  1. Load configuration from environment.
//  2. Initialize structured logger.
//  3. Ensure data dir exists; load or generate self-signed TLS material.
//  4. Open SQLite, run migrations.
//  5. Load or generate the JWT signing key.
//  6. If users table is empty, mint a one-time setup token and log it.
//  7. Build the Fiber router and serve until SIGINT/SIGTERM.
package main

import (
	"context"
	crtls "crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/api"
	coreauth "github.com/tm4rtin17/controlroom/internal/auth"
	"github.com/tm4rtin17/controlroom/internal/buildinfo"
	"github.com/tm4rtin17/controlroom/internal/cert"
	"github.com/tm4rtin17/controlroom/internal/collectors"
	"github.com/tm4rtin17/controlroom/internal/config"
	"github.com/tm4rtin17/controlroom/internal/docker"
	"github.com/tm4rtin17/controlroom/internal/jobs"
	"github.com/tm4rtin17/controlroom/internal/store"
	"github.com/tm4rtin17/controlroom/internal/systemd"
)

const shutdownGrace = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logger := newLogger(cfg.LogLevel)

	logger.Info().
		Str("version", buildinfo.Version).
		Str("commit", buildinfo.Commit).
		Str("date", buildinfo.Date).
		Str("addr", cfg.Addr).
		Str("data_dir", cfg.DataDir).
		Str("tls_mode", string(cfg.TLSMode)).
		Bool("trust_proxy", cfg.TrustProxy).
		Bool("dev_mode", cfg.DevMode).
		Msg("controlroom starting")

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("data dir %s not writable: %w", cfg.DataDir, err)
	}

	db, err := store.Open(filepath.Join(cfg.DataDir, "controlroom.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := bootstrapSetupToken(db, logger); err != nil {
		return fmt.Errorf("bootstrap setup token: %w", err)
	}

	signer, err := coreauth.LoadOrCreateSigner(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("jwt signer: %w", err)
	}

	// systemd is best-effort: in dev environments without a system bus
	// (containers, macOS) we boot anyway and let /api/services return 503.
	sysd, sysdErr := systemd.NewDBus(context.Background())
	if sysdErr != nil {
		logger.Warn().Err(sysdErr).Msg("systemd unavailable; /api/services disabled")
	}
	if sysd != nil {
		defer func() { _ = sysd.Close() }()
	}

	// Docker is also best-effort. The configured CR_DOCKER_SOCK overrides
	// the default; setting it to "" disables the module entirely.
	var dock *docker.DockerClient
	if cfg.DockerSock != "" {
		var err error
		dock, err = docker.New(context.Background(), cfg.DockerSock)
		if err != nil {
			logger.Warn().Err(err).Msg("docker unavailable; /api/containers disabled")
			dock = nil
		}
	}
	if dock != nil {
		defer func() { _ = dock.Close() }()
	}

	deps := api.Deps{
		Cfg:         cfg,
		Logger:      logger,
		DB:          db,
		Signer:      signer,
		Sessions:    coreauth.NewManager(db),
		Cookies:     coreauth.CookieOpts{DevMode: cfg.DevMode},
		IPLimiter:   coreauth.NewIPLimiter(5),
		UserBackoff: coreauth.NewUserBackoff(),
		Aggregator:  collectors.NewAggregator(),
		SystemD:     systemdOrNil(sysd),
		Docker:      dockerOrNil(dock),
		Jobs:        jobs.NewRunner(),
	}

	app := api.NewRouter(deps)

	ln, acmeHandler, err := buildListener(cfg)
	if err != nil {
		return err
	}
	logger.Info().Str("addr", ln.Addr().String()).Msg("listening")

	// ACME mode requires :80 to serve HTTP-01 challenges + redirect to HTTPS.
	var acmeSrv *http.Server
	if acmeHandler != nil {
		acmeSrv = &http.Server{
			Addr:              ":80",
			Handler:           acmeHandler,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			logger.Info().Str("addr", ":80").Msg("acme http-01 listening")
			if err := acmeSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error().Err(err).Msg("acme listener failed")
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srvErr := make(chan error, 1)
	go func() {
		if err := app.Listener(ln); err != nil &&
			!errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			srvErr <- err
			return
		}
		srvErr <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info().Msg("signal received; draining")
	case err := <-srvErr:
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if acmeSrv != nil {
		_ = acmeSrv.Shutdown(shutdownCtx)
	}
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info().Msg("controlroom stopped")
	return nil
}

func buildListener(cfg *config.Config) (net.Listener, http.Handler, error) {
	if cfg.DevMode {
		// Dev: plain HTTP. Cookie Secure flag is also dropped (CookieOpts.DevMode).
		ln, err := net.Listen("tcp", cfg.Addr)
		return ln, nil, err
	}
	tlsCfg, acmeHandler, err := cert.LoadOrGenerate(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("tls setup: %w", err)
	}
	ln, err := crtls.Listen("tcp", cfg.Addr, tlsCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("listen %s: %w", cfg.Addr, err)
	}
	return ln, acmeHandler, nil
}

// bootstrapSetupToken creates a fresh one-time token whenever the users table
// is empty (fresh install or a wiped DB). The raw token is logged once with an
// eye-catching banner; only its sha256 lives in the database.
func bootstrapSetupToken(db *store.DB, logger zerolog.Logger) error {
	ctx := context.Background()
	n, err := db.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		// Admin already exists — clean up any leftover token.
		_ = db.DeleteSetupToken(ctx)
		return nil
	}

	raw, hash, err := coreauth.MintSetupToken()
	if err != nil {
		return fmt.Errorf("mint token: %w", err)
	}
	if err := db.PutSetupToken(ctx, hash); err != nil {
		return fmt.Errorf("store token: %w", err)
	}

	const banner = "==================================================================="
	logger.Info().Msg(banner)
	logger.Info().Msg("ControlRoom first-run setup token (paste this in the browser):")
	logger.Info().Str("token", raw).Msg("setup_token")
	logger.Info().Msg(banner)
	return nil
}

// systemdOrNil returns nil for nil-typed *DBusClient so the api.Deps field
// (which is the systemd.Client interface) compares == nil correctly. Without
// this shim, `var c systemd.Client = (*systemd.DBusClient)(nil)` is non-nil.
func systemdOrNil(c *systemd.DBusClient) systemd.Client {
	if c == nil {
		return nil
	}
	return c
}

func dockerOrNil(c *docker.DockerClient) docker.Client {
	if c == nil {
		return nil
	}
	return c
}

func newLogger(level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil || lvl == zerolog.NoLevel {
		lvl = zerolog.InfoLevel
	}
	return zerolog.New(os.Stderr).
		Level(lvl).
		With().
		Timestamp().
		Str("svc", "controlroom").
		Logger()
}
