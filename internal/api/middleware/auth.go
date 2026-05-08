package middleware

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/tm4rtin17/controlroom/internal/auth"
	"github.com/tm4rtin17/controlroom/internal/store"
)

// Context keys for c.Locals. Exported so handlers in other packages can read
// the user/claims attached by RequireAuth.
const (
	CtxUser    = "cr.user"
	CtxClaims  = "cr.access_claims"
	CtxSession = "cr.session_id"
)

// RequireAuth verifies the access cookie, loads the user, and attaches both
// to the request locals. On any failure it returns 401 with code auth.unauthorized.
func RequireAuth(signer *auth.Signer, db *store.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Cookies(auth.CookieAccess)
		if raw == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing access token")
		}
		claims, err := signer.ParseAccess(raw)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid access token")
		}

		// Validate the session is still active. This costs one indexed lookup
		// per authenticated request — acceptable for a homelab; revisit if
		// load grows.
		sess, err := db.SessionByID(context.Background(), claims.SessionID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fiber.NewError(fiber.StatusUnauthorized, "session not found")
			}
			return fiber.NewError(fiber.StatusInternalServerError, "session lookup failed")
		}
		if sess.Revoked() {
			return fiber.NewError(fiber.StatusUnauthorized, "session revoked")
		}

		user, err := db.UserByID(context.Background(), claims.UserID)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "user not found")
		}

		c.Locals(CtxUser, user)
		c.Locals(CtxClaims, claims)
		c.Locals(CtxSession, claims.SessionID)
		return c.Next()
	}
}

// CurrentUser returns the user attached by RequireAuth, or nil if absent.
func CurrentUser(c *fiber.Ctx) *store.User {
	u, _ := c.Locals(CtxUser).(*store.User)
	return u
}

func CurrentSessionID(c *fiber.Ctx) string {
	s, _ := c.Locals(CtxSession).(string)
	return s
}
