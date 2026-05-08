// Package containers serves /api/containers/* and /ws/containers/*.
//
// Container IDs accepted from URLs are validated against a strict regex
// before being passed to the Docker client to prevent any pathological values
// from reaching the daemon.
package containers

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	"github.com/tm4rtin17/controlroom/internal/docker"
	"github.com/tm4rtin17/controlroom/internal/store"
)

type Deps struct {
	Client docker.Client // nil → 503
	DB     *store.DB
	Logger zerolog.Logger
}

const (
	stopGrace      = 10 * time.Second
	wsLogWriteWait = 5 * time.Second
	wsLogReadWait  = 70 * time.Second
	wsLogPing      = 30 * time.Second
)

// containerIDRE matches Docker's full or short container IDs (hex, 12-64 chars).
var containerIDRE = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

func validID(s string) bool { return containerIDRE.MatchString(s) }

func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/containers")
	g.Get("/", d.listHandler)
	g.Get("/:id", d.inspectHandler)
	g.Post("/:id/start", d.actionHandler("start"))
	g.Post("/:id/stop", d.actionHandler("stop"))
	g.Post("/:id/restart", d.actionHandler("restart"))
	g.Delete("/:id", d.removeHandler)
}

func MountWS(wsGroup fiber.Router, d Deps) {
	g := wsGroup.Group("/containers")
	g.Get("/:id/logs", websocket.New(d.logsWS, websocket.Config{HandshakeTimeout: 5 * time.Second}))
	g.Get("/:id/stats", websocket.New(d.statsWS, websocket.Config{HandshakeTimeout: 5 * time.Second}))
}

// ---- handlers ----

func (d Deps) requireClient() error {
	if d.Client == nil {
		return fiber.NewError(http.StatusServiceUnavailable, "docker daemon not available")
	}
	return nil
}

type listResp struct {
	Containers []docker.Container `json:"containers"`
}

func (d Deps) listHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	all := c.QueryBool("all", true)
	list, err := d.Client.List(c.Context(), docker.ListOptions{All: all})
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list containers: "+err.Error())
	}
	return c.JSON(listResp{Containers: list})
}

func (d Deps) inspectHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	id := c.Params("id")
	if !validID(id) {
		return fiber.NewError(http.StatusBadRequest, "invalid container id")
	}
	detail, err := d.Client.Inspect(c.Context(), id)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "inspect: "+err.Error())
	}
	return c.JSON(detail)
}

func (d Deps) actionHandler(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := d.requireClient(); err != nil {
			return err
		}
		id := c.Params("id")
		if !validID(id) {
			return fiber.NewError(http.StatusBadRequest, "invalid container id")
		}

		var err error
		switch action {
		case "start":
			err = d.Client.Start(c.Context(), id)
		case "stop":
			err = d.Client.Stop(c.Context(), id, stopGrace)
		case "restart":
			err = d.Client.Restart(c.Context(), id, stopGrace)
		default:
			return fiber.NewError(http.StatusInternalServerError, "unknown action")
		}

		entry := store.AuditEntry{
			IP:      c.IP(),
			Action:  "containers." + action,
			Target:  id,
			Outcome: "success",
		}
		if u := middleware.CurrentUser(c); u != nil {
			entry.UserID = u.ID
		}
		if err != nil {
			entry.Outcome = "failure"
			entry.Detail = map[string]string{"error": err.Error()}
			_ = d.DB.WriteAudit(c.Context(), entry)
			return fiber.NewError(http.StatusInternalServerError, action+" failed: "+err.Error())
		}
		_ = d.DB.WriteAudit(c.Context(), entry)
		return c.JSON(fiber.Map{"ok": true})
	}
}

func (d Deps) removeHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	id := c.Params("id")
	if !validID(id) {
		return fiber.NewError(http.StatusBadRequest, "invalid container id")
	}
	force := c.QueryBool("force", false)

	entry := store.AuditEntry{
		IP:      c.IP(),
		Action:  "containers.remove",
		Target:  id,
		Outcome: "success",
		Detail:  map[string]bool{"force": force},
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}

	if err := d.Client.Remove(c.Context(), id, force); err != nil {
		entry.Outcome = "failure"
		entry.Detail = map[string]any{"force": force, "error": err.Error()}
		_ = d.DB.WriteAudit(c.Context(), entry)
		return fiber.NewError(http.StatusInternalServerError, "remove: "+err.Error())
	}
	_ = d.DB.WriteAudit(c.Context(), entry)
	return c.JSON(fiber.Map{"ok": true})
}

// ---- websockets ----

type wsLogFrame struct {
	Type   string `json:"type"` // "line" | "error"
	Stream string `json:"stream,omitempty"`
	Line   string `json:"line,omitempty"`
	Err    string `json:"err,omitempty"`
}

func (d Deps) logsWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()
	if d.Client == nil {
		_ = c.WriteJSON(wsLogFrame{Type: "error", Err: "docker not available"})
		return
	}
	id := c.Params("id")
	if !validID(id) {
		_ = c.WriteJSON(wsLogFrame{Type: "error", Err: "invalid container id"})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = c.SetReadDeadline(time.Now().Add(wsLogReadWait))
	c.SetPongHandler(func(string) error { return c.SetReadDeadline(time.Now().Add(wsLogReadWait)) })

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	lines, err := d.Client.Logs(ctx, id, 200)
	if err != nil {
		_ = c.WriteJSON(wsLogFrame{Type: "error", Err: err.Error()})
		return
	}

	ping := time.NewTicker(wsLogPing)
	defer ping.Stop()

	for {
		select {
		case <-closed:
			return
		case <-ping.C:
			_ = c.SetWriteDeadline(time.Now().Add(wsLogWriteWait))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case line, ok := <-lines:
			if !ok {
				return
			}
			_ = c.SetWriteDeadline(time.Now().Add(wsLogWriteWait))
			if err := c.WriteJSON(wsLogFrame{Type: "line", Stream: line.Stream, Line: line.Line}); err != nil {
				return
			}
		}
	}
}

func (d Deps) statsWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()
	if d.Client == nil {
		return
	}
	id := c.Params("id")
	if !validID(id) {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = c.SetReadDeadline(time.Now().Add(wsLogReadWait))
	c.SetPongHandler(func(string) error { return c.SetReadDeadline(time.Now().Add(wsLogReadWait)) })

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	stats, err := d.Client.Stats(ctx, id)
	if err != nil {
		return
	}

	ping := time.NewTicker(wsLogPing)
	defer ping.Stop()

	for {
		select {
		case <-closed:
			return
		case <-ping.C:
			_ = c.SetWriteDeadline(time.Now().Add(wsLogWriteWait))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case s, ok := <-stats:
			if !ok {
				return
			}
			_ = c.SetWriteDeadline(time.Now().Add(wsLogWriteWait))
			if err := c.WriteJSON(s); err != nil {
				return
			}
		}
	}
}
