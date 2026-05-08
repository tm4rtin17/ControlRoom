package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// Cookie names. Path scoping limits where each cookie is sent. The access
// cookie is at "/" so it reaches both /api/* and /ws/* (the WebSocket
// upgrade is an HTTP request and needs the cookie attached). Refresh is
// pinned to /api/auth so it only ships with the routes that actually use it.
const (
	CookieAccess  = "cr_access"
	CookieRefresh = "cr_refresh"
	CookieCSRF    = "cr_csrf"
	CookieSetup   = "cr_setup"

	pathAuth  = "/api/auth"
	pathSetup = "/api/setup"
	pathRoot  = "/"
)

// CookieOpts is set once at construction; "Secure" is dropped only when DevMode
// is true so HTTP dev sessions work without a cert.
type CookieOpts struct {
	DevMode bool
}

func (o CookieOpts) base(name, value, path string, maxAge time.Duration, httpOnly bool) *fiber.Cookie {
	c := &fiber.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HTTPOnly: httpOnly,
		Secure:   !o.DevMode,
		SameSite: "Strict",
	}
	if maxAge > 0 {
		c.MaxAge = int(maxAge.Seconds())
		c.Expires = time.Now().Add(maxAge)
	} else if maxAge < 0 {
		// Negative => clear cookie now.
		c.MaxAge = -1
		c.Expires = time.Unix(0, 0)
	}
	return c
}

func (o CookieOpts) SetAccess(c *fiber.Ctx, token string) {
	c.Cookie(o.base(CookieAccess, token, pathRoot, AccessTokenTTL, true))
}

func (o CookieOpts) SetRefresh(c *fiber.Ctx, token string) {
	c.Cookie(o.base(CookieRefresh, token, pathAuth, RefreshTokenTTL, true))
}

func (o CookieOpts) SetCSRF(c *fiber.Ctx, token string) {
	// Not HTTPOnly: the SPA must read this and mirror it in the X-CSRF-Token
	// header for state-changing requests (double-submit pattern).
	c.Cookie(o.base(CookieCSRF, token, pathRoot, RefreshTokenTTL, false))
}

func (o CookieOpts) SetSetup(c *fiber.Ctx, token string) {
	c.Cookie(o.base(CookieSetup, token, pathSetup, SetupTokenTTL, true))
}

func (o CookieOpts) ClearAuth(c *fiber.Ctx) {
	c.Cookie(o.base(CookieAccess, "", pathRoot, -1, true))
	c.Cookie(o.base(CookieRefresh, "", pathAuth, -1, true))
	c.Cookie(o.base(CookieCSRF, "", pathRoot, -1, false))
}

func (o CookieOpts) ClearSetup(c *fiber.Ctx) {
	c.Cookie(o.base(CookieSetup, "", pathSetup, -1, true))
}
