// Package middleware contains shared HTTP middleware (logging, request-id, etc.).
package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
)

// Logger emits one structured line per request after the handler runs.
func Logger(logger zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()

		evt := logger.Info()
		switch {
		case status >= 500 || err != nil:
			evt = logger.Error().Err(err)
		case status >= 400:
			evt = logger.Warn()
		}

		reqID, _ := c.Locals(requestid.ConfigDefault.ContextKey).(string)
		evt.
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Str("ip", c.IP()).
			Str("request_id", reqID).
			Msg("http")
		return err
	}
}
