package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/tm4rtin17/controlroom/internal/auth"
)

const csrfHeader = "X-CSRF-Token"

// CSRF enforces the double-submit cookie pattern: the SPA reads cr_csrf
// (non-HTTPOnly) and mirrors it in the X-CSRF-Token header for any state-
// changing request. Mismatch → 403.
//
// GET/HEAD/OPTIONS are skipped: those methods are safe and the SPA's first
// load wouldn't yet have a CSRF cookie to mirror.
func CSRF() fiber.Handler {
	return func(c *fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}
		cookie := c.Cookies(auth.CookieCSRF)
		header := c.Get(csrfHeader)
		if cookie == "" || header == "" || !auth.ConstantTimeEqualBytes([]byte(cookie), []byte(header)) {
			return fiber.NewError(fiber.StatusForbidden, "csrf token mismatch")
		}
		return c.Next()
	}
}
