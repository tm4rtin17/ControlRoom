// Package api wires HTTP routes, middleware, and handlers.
package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"

	authapi "github.com/tm4rtin17/controlroom/internal/api/auth"
	containersapi "github.com/tm4rtin17/controlroom/internal/api/containers"
	k8sapi "github.com/tm4rtin17/controlroom/internal/api/k8s"
	logsapi "github.com/tm4rtin17/controlroom/internal/api/logs"
	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	networkapi "github.com/tm4rtin17/controlroom/internal/api/network"
	servicesapi "github.com/tm4rtin17/controlroom/internal/api/services"
	settingsapi "github.com/tm4rtin17/controlroom/internal/api/settings"
	setupapi "github.com/tm4rtin17/controlroom/internal/api/setup"
	systemapi "github.com/tm4rtin17/controlroom/internal/api/system"
	terminalapi "github.com/tm4rtin17/controlroom/internal/api/terminal"
	updatesapi "github.com/tm4rtin17/controlroom/internal/api/updates"
	coreauth "github.com/tm4rtin17/controlroom/internal/auth"
	"github.com/tm4rtin17/controlroom/internal/buildinfo"
	"github.com/tm4rtin17/controlroom/internal/collectors"
	"github.com/tm4rtin17/controlroom/internal/config"
	"github.com/tm4rtin17/controlroom/internal/docker"
	"github.com/tm4rtin17/controlroom/internal/jobs"
	"github.com/tm4rtin17/controlroom/internal/k8s"
	"github.com/tm4rtin17/controlroom/internal/store"
	"github.com/tm4rtin17/controlroom/internal/systemd"
	"github.com/tm4rtin17/controlroom/internal/web"
)

// Deps bundles every dependency the router needs to wire a request through.
type Deps struct {
	Cfg         *config.Config
	Logger      zerolog.Logger
	DB          *store.DB
	Signer      *coreauth.Signer
	Sessions    *coreauth.Manager
	Cookies     coreauth.CookieOpts
	IPLimiter   *coreauth.IPLimiter
	UserBackoff *coreauth.UserBackoff
	Aggregator  *collectors.Aggregator
	SystemD     systemd.Client // nil → /api/services returns 503
	Docker      docker.Client  // nil → /api/containers returns 503
	K8s         *k8s.Client    // nil → /api/k8s returns 503
	Jobs        *jobs.Runner
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

// NewRouter builds the Fiber app with all middleware and routes registered.
func NewRouter(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "controlroom",
		DisableStartupMessage:   true,
		BodyLimit:               1 << 20, // 1 MiB
		EnableTrustedProxyCheck: d.Cfg.TrustProxy,
		ProxyHeader:             proxyHeader(d.Cfg.TrustProxy),
		ErrorHandler:            errorHandler,
	})

	app.Use(requestid.New())
	app.Use(fiberrecover.New())
	app.Use(middleware.Logger(d.Logger))

	api := app.Group("/api")
	api.Get("/healthz", healthz)
	api.Get("/version", version)

	// First-run wizard.
	setupapi.Mount(api, setupapi.Deps{
		DB: d.DB, Signer: d.Signer, Sessions: d.Sessions,
		Cookies: d.Cookies, Issuer: "ControlRoom",
	})

	// Auth: public routes (login, refresh).
	authDeps := authapi.Deps{
		DB: d.DB, Signer: d.Signer, Sessions: d.Sessions,
		Cookies: d.Cookies, IPLimiter: d.IPLimiter,
		UserBackoff: d.UserBackoff, Issuer: "ControlRoom",
	}
	authapi.MountPublic(api, authDeps)

	// Authenticated + CSRF group.
	guarded := api.Group("",
		middleware.RequireAuth(d.Signer, d.DB),
		middleware.CSRF(),
	)
	authapi.MountAuthenticated(guarded, authDeps)

	systemDeps := systemapi.Deps{
		Aggregator: d.Aggregator,
		Logger:     d.Logger,
		SystemD:    d.SystemD,
		Docker:     d.Docker,
		K8s:        d.K8s,
	}
	systemapi.MountHTTP(guarded, systemDeps)

	servicesDeps := servicesapi.Deps{Client: d.SystemD, DB: d.DB, Logger: d.Logger}
	servicesapi.MountHTTP(guarded, servicesDeps)

	containersDeps := containersapi.Deps{Client: d.Docker, DB: d.DB, Logger: d.Logger}
	containersapi.MountHTTP(guarded, containersDeps)

	updatesDeps := updatesapi.Deps{Runner: d.Jobs, DB: d.DB, Logger: d.Logger}
	updatesapi.MountHTTP(guarded, updatesDeps)
	updatesapi.MountReboot(guarded, updatesDeps)

	networkapi.MountHTTP(guarded, networkapi.Deps{DB: d.DB, Logger: d.Logger})
	logsapi.MountHTTP(guarded, logsapi.Deps{Logger: d.Logger})
	k8sapi.MountHTTP(guarded, k8sapi.Deps{Client: d.K8s, Logger: d.Logger})
	settingsapi.MountHTTP(guarded, settingsapi.Deps{Cfg: d.Cfg, DB: d.DB})

	// Catch-all 404 for unknown /api paths.
	api.All("/*", notFound)

	// WebSocket group: auth via cookie, no CSRF (handshake is GET).
	wsGroup := app.Group("/ws",
		// Pre-flight: only allow upgrade requests through.
		websocketUpgradeGuard,
		middleware.RequireAuth(d.Signer, d.DB),
	)
	systemapi.MountWS(wsGroup, systemDeps)
	servicesapi.MountWS(wsGroup, servicesDeps)
	containersapi.MountWS(wsGroup, containersDeps)
	terminalapi.MountWS(wsGroup, terminalapi.Deps{
		DB:            d.DB,
		Logger:        d.Logger,
		HostShell:     d.Cfg.HostShell,
		TerminalLogin: d.Cfg.TerminalLogin,
	})
	updatesapi.MountWS(wsGroup, updatesDeps)
	logsapi.MountWS(wsGroup, logsapi.Deps{Logger: d.Logger})
	k8sapi.MountWS(wsGroup, k8sapi.Deps{Client: d.K8s, Logger: d.Logger})

	wsGroup.All("/*", notFound)

	// SPA fallback — last.
	web.Mount(app)
	return app
}

// websocketUpgradeGuard rejects non-WS GETs on /ws/* with 426 so we don't
// expose websocket upgrades behind plain HTTP misuse.
func websocketUpgradeGuard(c *fiber.Ctx) error {
	if strings.EqualFold(c.Get(fiber.HeaderUpgrade), "websocket") {
		return c.Next()
	}
	return fiber.NewError(fiber.StatusUpgradeRequired, "websocket upgrade required")
}

func healthz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func version(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
		"date":    buildinfo.Date,
	})
}

func notFound(c *fiber.Ctx) error {
	return fiber.ErrNotFound
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var fe *fiber.Error
	if errors.As(err, &fe) {
		code = fe.Code
	}
	return c.Status(code).JSON(errorBody{
		Error: errorDetail{
			Code:    httpCodeName(code),
			Message: err.Error(),
		},
	})
}

func proxyHeader(trust bool) string {
	if trust {
		return fiber.HeaderXForwardedFor
	}
	return ""
}

func httpCodeName(code int) string {
	switch code {
	case 400:
		return "bad_request"
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 404:
		return "not_found"
	case 405:
		return "method_not_allowed"
	case 409:
		return "conflict"
	case 410:
		return "gone"
	case 413:
		return "payload_too_large"
	case 426:
		return "upgrade_required"
	case 429:
		return "rate_limited"
	case 503:
		return "unavailable"
	default:
		return "internal"
	}
}
