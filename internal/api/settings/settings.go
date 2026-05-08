// Package settings serves /api/settings/* — read-only server config + change-password.
//
// v0.1 ships read-only settings backed by env. Persisted preferences (host
// display name, telemetry opt-in toggle from the UI) are deferred to a future
// release; everything that needs to be configurable goes through environment
// variables today.
package settings

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	coreauth "github.com/tm4rtin17/controlroom/internal/auth"
	"github.com/tm4rtin17/controlroom/internal/buildinfo"
	"github.com/tm4rtin17/controlroom/internal/config"
	"github.com/tm4rtin17/controlroom/internal/store"
)

type Deps struct {
	Cfg *config.Config
	DB  *store.DB
}

func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/settings")
	g.Get("/", d.getHandler)
	g.Post("/password", d.changePasswordHandler)
}

type settingsResp struct {
	Server  serverInfo  `json:"server"`
	TLS     tlsInfo     `json:"tls"`
	Build   buildInfo   `json:"build"`
}

type serverInfo struct {
	Addr         string `json:"addr"`
	HostName     string `json:"host_name,omitempty"`
	LogLevel     string `json:"log_level"`
	DevMode      bool   `json:"dev_mode"`
	TrustProxy   bool   `json:"trust_proxy"`
	VersionCheck bool   `json:"version_check"`
	SessionHours int    `json:"session_hours"`
}

type tlsInfo struct {
	Mode     string `json:"mode"`
	ACMEHost string `json:"acme_host,omitempty"`
}

type buildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func (d Deps) getHandler(c *fiber.Ctx) error {
	return c.JSON(settingsResp{
		Server: serverInfo{
			Addr:         d.Cfg.Addr,
			HostName:     d.Cfg.HostName,
			LogLevel:     d.Cfg.LogLevel,
			DevMode:      d.Cfg.DevMode,
			TrustProxy:   d.Cfg.TrustProxy,
			VersionCheck: d.Cfg.VersionCheck,
			SessionHours: d.Cfg.SessionHours,
		},
		TLS:   tlsInfo{Mode: string(d.Cfg.TLSMode), ACMEHost: d.Cfg.ACMEHost},
		Build: buildInfo{Version: buildinfo.Version, Commit: buildinfo.Commit, Date: buildinfo.Date},
	})
}

type changePasswordReq struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (d Deps) changePasswordHandler(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	if user == nil {
		return fiber.NewError(http.StatusUnauthorized, "not authenticated")
	}
	var req changePasswordReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if !coreauth.VerifyPassword(user.PasswordHash, req.Current) {
		_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
			UserID: user.ID, IP: c.IP(), Action: "settings.change_password", Outcome: "failure",
			Detail: map[string]string{"reason": "wrong_current"},
		})
		return fiber.NewError(http.StatusUnauthorized, "current password incorrect")
	}
	hash, err := coreauth.HashPassword(req.New)
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	if err := d.DB.UpdatePassword(c.Context(), user.ID, hash); err != nil {
		return fiber.NewError(http.StatusInternalServerError, "update password")
	}
	// Revoke every other session to invalidate any stolen cookies.
	_ = d.DB.RevokeAllForUser(c.Context(), user.ID)

	_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
		UserID: user.ID, IP: c.IP(), Action: "settings.change_password", Outcome: "success",
		Detail: map[string]any{"at": time.Now().UTC().Format(time.RFC3339)},
	})
	return c.JSON(fiber.Map{"ok": true})
}
