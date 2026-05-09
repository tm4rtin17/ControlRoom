// Package auth (in api/) holds the HTTP handlers for /api/auth/*.
// It depends on internal/auth for primitives and internal/store for data.
package auth

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	coreauth "github.com/tm4rtin17/controlroom/internal/auth"
	"github.com/tm4rtin17/controlroom/internal/store"
)

type Deps struct {
	DB         *store.DB
	Signer     *coreauth.Signer
	Sessions   *coreauth.Manager
	Cookies    coreauth.CookieOpts
	IPLimiter  *coreauth.IPLimiter   // login per-IP
	UserBackoff *coreauth.UserBackoff // login per-user
	Issuer     string
}

// MountPublic registers the routes that work without an existing session.
// These endpoints are explicitly outside the CSRF guard because the client has
// no session yet to mirror a token from.
func MountPublic(api fiber.Router, d Deps) {
	g := api.Group("/auth")
	g.Post("/login", d.loginHandler)
	g.Post("/refresh", d.refreshHandler)
}

// MountAuthenticated registers the routes that require an existing session.
// Caller is responsible for applying RequireAuth + CSRF middleware to the
// router passed in.
func MountAuthenticated(authed fiber.Router, d Deps) {
	g := authed.Group("/auth")
	g.Post("/logout", d.logoutHandler)
	g.Get("/me", d.meHandler)
	g.Post("/2fa/enroll", d.totpEnrollHandler)
	g.Post("/2fa/verify", d.totpVerifyHandler)
	g.Post("/2fa/disable", d.totpDisableHandler)
}

// ---- login / logout / refresh ----

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTP     string `json:"totp,omitempty"`
}

func (d Deps) loginHandler(c *fiber.Ctx) error {
	if ok, retry := d.IPLimiter.Allow(c.IP()); !ok {
		c.Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
		return fiber.NewError(http.StatusTooManyRequests, "too many login attempts from this IP")
	}

	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if req.Username == "" || req.Password == "" {
		return fiber.NewError(http.StatusBadRequest, "username and password required")
	}

	if ok, retry := d.UserBackoff.CanAttempt(req.Username); !ok {
		c.Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
		return fiber.NewError(http.StatusTooManyRequests, "too many failed attempts; try again later")
	}

	user, err := d.DB.UserByUsername(c.Context(), req.Username)
	if err != nil {
		// Run a dummy bcrypt to keep login timing roughly equal between
		// existing-user-wrong-password and unknown-user.
		_ = coreauth.VerifyPassword([]byte("$2a$12$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvalid"), req.Password)
		_ = d.UserBackoff.Failure(req.Username)
		_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
			IP: c.IP(), Action: "auth.login", Outcome: "failure",
			Detail: map[string]any{"username": req.Username, "reason": "unknown_user"},
		})
		return fiber.NewError(http.StatusUnauthorized, "invalid credentials")
	}

	if !coreauth.VerifyPassword(user.PasswordHash, req.Password) {
		_ = d.UserBackoff.Failure(req.Username)
		_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
			UserID: user.ID, IP: c.IP(), Action: "auth.login", Outcome: "failure",
			Detail: map[string]any{"reason": "bad_password"},
		})
		return fiber.NewError(http.StatusUnauthorized, "invalid credentials")
	}

	if user.TOTPEnabled {
		if req.TOTP == "" {
			return fiber.NewError(http.StatusUnauthorized, "totp required")
		}
		if !coreauth.VerifyTOTP(user.TOTPSecret, req.TOTP) {
			_ = d.UserBackoff.Failure(req.Username)
			_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
				UserID: user.ID, IP: c.IP(), Action: "auth.login", Outcome: "failure",
				Detail: map[string]any{"reason": "bad_totp"},
			})
			return fiber.NewError(http.StatusUnauthorized, "invalid credentials")
		}
	}

	// Success — clear backoff state for this username.
	d.UserBackoff.Success(req.Username)

	if err := d.issueAndSetCookies(c, user); err != nil {
		return err
	}

	_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
		UserID: user.ID, IP: c.IP(), Action: "auth.login", Outcome: "success",
	})
	return c.JSON(meResponse(user))
}

func (d Deps) logoutHandler(c *fiber.Ctx) error {
	// Best-effort revoke whichever session the cookies point to.
	if raw := c.Cookies(coreauth.CookieRefresh); raw != "" {
		if rt, err := coreauth.ParseRefreshToken(raw); err == nil {
			_ = d.Sessions.Revoke(c.Context(), rt.SessionID)
		}
	}
	d.Cookies.ClearAuth(c)
	_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
		IP: c.IP(), Action: "auth.logout", Outcome: "success",
	})
	return c.JSON(fiber.Map{"ok": true})
}

func (d Deps) refreshHandler(c *fiber.Ctx) error {
	raw := c.Cookies(coreauth.CookieRefresh)
	if raw == "" {
		return fiber.NewError(http.StatusUnauthorized, "missing refresh token")
	}
	rt, err := coreauth.ParseRefreshToken(raw)
	if err != nil {
		return fiber.NewError(http.StatusUnauthorized, "malformed refresh token")
	}

	res, err := d.Sessions.Rotate(c.Context(), d.Signer, rt, c.IP(), c.Get(fiber.HeaderUserAgent))
	switch {
	case errors.Is(err, coreauth.ErrSessionReuse):
		d.Cookies.ClearAuth(c)
		_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
			IP: c.IP(), Action: "auth.refresh", Outcome: "failure",
			Detail: map[string]string{"reason": "reuse_detected_family_revoked"},
		})
		return fiber.NewError(http.StatusUnauthorized, "session reuse detected")
	case errors.Is(err, coreauth.ErrSessionInvalid):
		d.Cookies.ClearAuth(c)
		return fiber.NewError(http.StatusUnauthorized, "session invalid")
	case err != nil:
		return fiber.NewError(http.StatusInternalServerError, "rotate failed")
	}

	csrf, err := coreauth.MintCSRFToken()
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "csrf")
	}
	d.Cookies.SetAccess(c, res.AccessToken)
	d.Cookies.SetRefresh(c, res.RefreshToken.String())
	d.Cookies.SetCSRF(c, csrf)

	return c.JSON(fiber.Map{"ok": true, "expires_in": int(coreauth.AccessTokenTTL.Seconds())})
}

// ---- me ----

func (d Deps) meHandler(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return fiber.NewError(http.StatusUnauthorized, "not authenticated")
	}
	return c.JSON(meResponse(user))
}

// ---- 2FA management (post-login) ----

type totpEnrollResp struct {
	Secret    string `json:"secret"`
	URI       string `json:"uri"`
	QRDataURI string `json:"qr_data_uri"`
}

func (d Deps) totpEnrollHandler(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return fiber.NewError(http.StatusUnauthorized, "not authenticated")
	}
	if user.TOTPEnabled {
		return fiber.NewError(http.StatusConflict, "totp already enabled")
	}
	enrol, err := coreauth.GenerateTOTP(d.Issuer, user.Username)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "totp generate")
	}
	// We return the secret to the SPA; SPA must echo it back with the verify
	// code. This avoids holding partial state in the server.
	return c.JSON(totpEnrollResp{Secret: enrol.Secret, URI: enrol.URI, QRDataURI: enrol.QRDataURI})
}

type totpVerifyReq struct {
	Secret string `json:"secret"`
	Code   string `json:"code"`
}

func (d Deps) totpVerifyHandler(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return fiber.NewError(http.StatusUnauthorized, "not authenticated")
	}
	if user.TOTPEnabled {
		return fiber.NewError(http.StatusConflict, "totp already enabled")
	}
	var req totpVerifyReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if !coreauth.VerifyTOTP(req.Secret, req.Code) {
		return fiber.NewError(http.StatusBadRequest, "code did not match")
	}
	if err := d.DB.SetTOTP(c.Context(), user.ID, req.Secret, true); err != nil {
		return fiber.NewError(http.StatusInternalServerError, "store totp")
	}
	_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
		UserID: user.ID, IP: c.IP(), Action: "auth.2fa.enable", Outcome: "success",
	})
	return c.JSON(fiber.Map{"ok": true})
}

type totpDisableReq struct {
	Password string `json:"password"`
}

func (d Deps) totpDisableHandler(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return fiber.NewError(http.StatusUnauthorized, "not authenticated")
	}
	if !user.TOTPEnabled {
		return fiber.NewError(http.StatusConflict, "totp not enabled")
	}
	var req totpDisableReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if !coreauth.VerifyPassword(user.PasswordHash, req.Password) {
		return fiber.NewError(http.StatusUnauthorized, "password mismatch")
	}
	if err := d.DB.SetTOTP(c.Context(), user.ID, "", false); err != nil {
		return fiber.NewError(http.StatusInternalServerError, "update totp")
	}
	_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
		UserID: user.ID, IP: c.IP(), Action: "auth.2fa.disable", Outcome: "success",
	})
	return c.JSON(fiber.Map{"ok": true})
}

// ---- helpers ----

func getUser(c *fiber.Ctx) *store.User {
	return middleware.CurrentUser(c)
}

func (d Deps) issueAndSetCookies(c *fiber.Ctx, user *store.User) error {
	res, err := d.Sessions.Issue(c.Context(), d.Signer, user.ID, user.Role, c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "issue session")
	}
	csrf, err := coreauth.MintCSRFToken()
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "csrf")
	}
	d.Cookies.SetAccess(c, res.AccessToken)
	d.Cookies.SetRefresh(c, res.RefreshToken.String())
	d.Cookies.SetCSRF(c, csrf)
	return nil
}

type meResp struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	Role        string    `json:"role"`
	TOTPEnabled bool      `json:"totp_enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

func meResponse(u *store.User) fiber.Map {
	return fiber.Map{"user": meResp{
		ID: u.ID, Username: u.Username, Role: u.Role,
		TOTPEnabled: u.TOTPEnabled, CreatedAt: u.CreatedAt,
	}}
}
