// Package web embeds the built Vite SPA and mounts it under "/".
//
// Build order matters: `make web` populates web/dist before `go build` runs;
// the placeholder web/dist/index.html in version control keeps `go build` from
// failing on a fresh checkout.
package web

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"github.com/gofiber/fiber/v2"
)

//go:embed all:dist
var distFS embed.FS

func sub() fs.FS {
	s, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web/dist not embedded: " + err.Error())
	}
	return s
}

// Mount installs the SPA fallback as the last handler. Anything under /api or
// /ws that hasn't been claimed earlier returns 404 here (defensive — those
// prefixes should never resolve to index.html).
func Mount(app *fiber.App) {
	root := sub()

	app.Use(func(c *fiber.Ctx) error {
		p := c.Path()
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/ws") {
			return fiber.ErrNotFound
		}
		return serve(c, root, p)
	})
}

func serve(c *fiber.Ctx, root fs.FS, urlPath string) error {
	cleaned := strings.TrimPrefix(urlPath, "/")
	if cleaned == "" {
		cleaned = "index.html"
	}

	data, err := fs.ReadFile(root, cleaned)
	if err != nil {
		// SPA fallback: any unknown route serves index.html so client-side
		// routing can take over.
		data, err = fs.ReadFile(root, "index.html")
		if err != nil {
			return fiber.ErrNotFound
		}
		setCacheHeaders(c, "index.html")
		return c.Type("html").Send(data)
	}

	setCacheHeaders(c, cleaned)
	ext := strings.TrimPrefix(path.Ext(cleaned), ".")
	if ext == "" {
		ext = "html"
	}
	return c.Type(ext).Send(data)
}

func setCacheHeaders(c *fiber.Ctx, urlPath string) {
	switch {
	case strings.HasPrefix(urlPath, "assets/"):
		// Vite emits content-hashed filenames under assets/ — safe to cache forever.
		c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
	default:
		// Everything else (index.html, favicon, root assets) must revalidate.
		c.Set(fiber.HeaderCacheControl, "no-cache")
	}
}
