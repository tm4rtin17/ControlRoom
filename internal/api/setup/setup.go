// Package setup implements the first-run wizard endpoints.
//
// Lifecycle: while the users table is empty, /api/setup/status returns
// {required: true} and the SPA routes to /setup. The operator pastes the
// one-time token (printed once at boot), the server issues a short-lived
// setup JWT, and the wizard then submits the admin credentials. After
// completion these routes return 410 Gone — the wizard is one-shot.
package setup

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/tm4rtin17/controlroom/internal/auth"
	"github.com/tm4rtin17/controlroom/internal/store"
)

type Deps struct {
	DB         *store.DB
	Signer     *auth.Signer
	Sessions   *auth.Manager
	Cookies    auth.CookieOpts
	Issuer     string // for TOTP enrollment label
}

func Mount(api fiber.Router, d Deps) {
	g := api.Group("/setup")
	g.Get("/status", d.statusHandler)
	g.Post("/verify-token", d.verifyTokenHandler)
	g.Post("/2fa/preview", d.requireSetup, d.totpPreviewHandler)
	g.Post("/complete", d.requireSetup, d.completeHandler)
}

// ---- handlers ----

type statusResp struct {
	Required bool `json:"required"`
}

func (d Deps) statusHandler(c *fiber.Ctx) error {
	required, err := d.setupRequired(c.Context())
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "status check failed")
	}
	return c.JSON(statusResp{Required: required})
}

type verifyTokenReq struct {
	Token string `json:"token"`
}

func (d Deps) verifyTokenHandler(c *fiber.Ctx) error {
	if err := d.requireSetupRequired(c.Context()); err != nil {
		return err
	}

	var req verifyTokenReq
	if err := c.BodyParser(&req); err != nil || req.Token == "" {
		return fiber.NewError(http.StatusBadRequest, "missing token")
	}

	tok, err := d.DB.GetSetupToken(c.Context())
	if err != nil {
		_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
			IP: c.IP(), Action: "setup.verify_token", Outcome: "failure",
			Detail: map[string]string{"reason": "no_setup_token"},
		})
		return fiber.NewError(http.StatusUnauthorized, "invalid token")
	}
	if tok.Used() {
		return fiber.NewError(http.StatusGone, "setup token already used")
	}

	if !auth.ConstantTimeEqualBytes(tok.TokenHash, auth.HashToken(req.Token)) {
		_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
			IP: c.IP(), Action: "setup.verify_token", Outcome: "failure",
		})
		return fiber.NewError(http.StatusUnauthorized, "invalid token")
	}

	jwt, err := d.Signer.IssueSetup()
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "issue setup token")
	}
	d.Cookies.SetSetup(c, jwt)
	_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
		IP: c.IP(), Action: "setup.verify_token", Outcome: "success",
	})
	return c.JSON(fiber.Map{"ok": true})
}

type previewResp struct {
	Secret    string `json:"secret"`
	URI       string `json:"uri"`
	QRDataURI string `json:"qr_data_uri"`
}

type previewReq struct {
	Username string `json:"username"`
}

func (d Deps) totpPreviewHandler(c *fiber.Ctx) error {
	var req previewReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if !validUsername(req.Username) {
		return fiber.NewError(http.StatusBadRequest, "invalid username")
	}
	enrol, err := auth.GenerateTOTP(d.Issuer, req.Username)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "totp generate")
	}
	return c.JSON(previewResp{Secret: enrol.Secret, URI: enrol.URI, QRDataURI: enrol.QRDataURI})
}

type completeReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTP     *struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	} `json:"totp,omitempty"`
}

func (d Deps) completeHandler(c *fiber.Ctx) error {
	if err := d.requireSetupRequired(c.Context()); err != nil {
		return err
	}

	var req completeReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if !validUsername(req.Username) {
		return fiber.NewError(http.StatusBadRequest, "username must be 3-32 chars: letters, digits, underscore, dash")
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	var totpSecret string
	totpEnabled := false
	if req.TOTP != nil {
		if req.TOTP.Secret == "" || req.TOTP.Code == "" {
			return fiber.NewError(http.StatusBadRequest, "totp secret and code required")
		}
		if !auth.VerifyTOTP(req.TOTP.Secret, req.TOTP.Code) {
			return fiber.NewError(http.StatusBadRequest, "totp code did not match")
		}
		totpSecret = req.TOTP.Secret
		totpEnabled = true
	}

	user, err := d.DB.CreateUser(c.Context(), store.CreateUserParams{
		Username:     req.Username,
		PasswordHash: hash,
		TOTPSecret:   totpSecret,
		TOTPEnabled:  totpEnabled,
		Role:         "admin",
	})
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "create admin user")
	}
	if err := d.DB.MarkSetupTokenUsed(c.Context()); err != nil {
		// non-fatal; we already created the user
		_ = err
	}

	// Log the new admin in via cookies so they don't see /login next.
	res, err := d.Sessions.Issue(c.Context(), d.Signer, user.ID, user.Role, c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "issue session")
	}
	csrf, err := auth.MintCSRFToken()
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "csrf")
	}
	d.Cookies.SetAccess(c, res.AccessToken)
	d.Cookies.SetRefresh(c, res.RefreshToken.String())
	d.Cookies.SetCSRF(c, csrf)
	d.Cookies.ClearSetup(c)

	_ = d.DB.WriteAudit(c.Context(), store.AuditEntry{
		UserID: user.ID, IP: c.IP(), Action: "setup.complete", Outcome: "success",
		Detail: map[string]any{"totp_enabled": totpEnabled},
	})

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":           user.ID,
			"username":     user.Username,
			"role":         user.Role,
			"totp_enabled": user.TOTPEnabled,
		},
	})
}

// ---- helpers ----

func (d Deps) setupRequired(ctx context.Context) (bool, error) {
	n, err := d.DB.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

func (d Deps) requireSetupRequired(ctx context.Context) error {
	required, err := d.setupRequired(ctx)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "setup state")
	}
	if !required {
		return fiber.NewError(http.StatusGone, "setup already complete")
	}
	return nil
}

// requireSetup gates 2fa/preview and complete behind the short-lived setup JWT.
func (d Deps) requireSetup(c *fiber.Ctx) error {
	raw := c.Cookies(auth.CookieSetup)
	if raw == "" {
		return fiber.NewError(http.StatusUnauthorized, "missing setup token")
	}
	if _, err := d.Signer.ParseSetup(raw); err != nil {
		return fiber.NewError(http.StatusUnauthorized, "invalid setup token")
	}
	return c.Next()
}
