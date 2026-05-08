// Package services serves /api/services/* and /ws/services/:unit/logs.
//
// All write operations are audited. Unit names are validated against
// systemd.ValidUnitName before being passed to the dbus client; invalid names
// short-circuit with 400.
package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	"github.com/tm4rtin17/controlroom/internal/store"
	"github.com/tm4rtin17/controlroom/internal/systemd"
)

type Deps struct {
	Client systemd.Client // nil means "module disabled" — handlers return 503
	DB     *store.DB
	Logger zerolog.Logger
}

// MountHTTP registers REST endpoints on the authenticated group.
func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/services")
	g.Get("/", d.listHandler)
	g.Get("/:unit", d.getHandler)
	g.Post("/:unit/start", d.actionHandler("start"))
	g.Post("/:unit/stop", d.actionHandler("stop"))
	g.Post("/:unit/restart", d.actionHandler("restart"))
	g.Post("/:unit/enable", d.actionHandler("enable"))
	g.Post("/:unit/disable", d.actionHandler("disable"))
}

// MountWS registers the live log endpoint on the WS group.
func MountWS(wsGroup fiber.Router, d Deps) {
	g := wsGroup.Group("/services")
	g.Get("/:unit/logs", websocket.New(d.logsWS, websocket.Config{
		HandshakeTimeout: 5 * time.Second,
	}))
}

// ---- handlers ----

func (d Deps) requireClient(c *fiber.Ctx) error {
	if d.Client == nil {
		return fiber.NewError(http.StatusServiceUnavailable, "systemd not available on this host")
	}
	return nil
}

type listResp struct {
	Units []systemd.Unit `json:"units"`
}

func (d Deps) listHandler(c *fiber.Ctx) error {
	if err := d.requireClient(c); err != nil {
		return err
	}

	filter := systemd.ListFilter{
		Search: c.Query("q"),
	}
	if t := c.Query("type"); t != "" {
		filter.Suffixes = strings.Split(t, ",")
	}
	if s := c.Query("state"); s != "" {
		filter.States = strings.Split(s, ",")
	}

	units, err := d.Client.List(c.Context(), filter)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list units: "+err.Error())
	}
	return c.JSON(listResp{Units: units})
}

func (d Deps) getHandler(c *fiber.Ctx) error {
	if err := d.requireClient(c); err != nil {
		return err
	}
	name := c.Params("unit")
	if !systemd.ValidUnitName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid unit name")
	}
	detail, err := d.Client.Get(c.Context(), name)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "get unit: "+err.Error())
	}
	return c.JSON(detail)
}

func (d Deps) actionHandler(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := d.requireClient(c); err != nil {
			return err
		}
		name := c.Params("unit")
		if !systemd.ValidUnitName(name) {
			return fiber.NewError(http.StatusBadRequest, "invalid unit name")
		}

		var fn func(context.Context, string) error
		switch action {
		case "start":
			fn = d.Client.Start
		case "stop":
			fn = d.Client.Stop
		case "restart":
			fn = d.Client.Restart
		case "enable":
			fn = d.Client.Enable
		case "disable":
			fn = d.Client.Disable
		default:
			return fiber.NewError(http.StatusInternalServerError, "unknown action")
		}

		err := fn(c.Context(), name)
		entry := store.AuditEntry{
			IP:      c.IP(),
			Action:  "services." + action,
			Target:  name,
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

// ---- websocket ----

const (
	wsLogWriteWait = 5 * time.Second
	wsLogReadWait  = 70 * time.Second
	wsLogPing      = 30 * time.Second
)

type logFrame struct {
	Type string `json:"type"` // "line" | "error"
	Line string `json:"line,omitempty"`
	Err  string `json:"err,omitempty"`
}

func (d Deps) logsWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	if d.Client == nil {
		_ = c.WriteJSON(logFrame{Type: "error", Err: "systemd not available"})
		return
	}

	name := c.Params("unit")
	if !systemd.ValidUnitName(name) {
		_ = c.WriteJSON(logFrame{Type: "error", Err: "invalid unit name"})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = c.SetReadDeadline(time.Now().Add(wsLogReadWait))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(wsLogReadWait))
	})

	// Drain client messages so reads keep advancing the deadline; close on
	// any read error (including peer close).
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	lines, err := systemd.TailUnit(ctx, name, 200)
	if err != nil {
		_ = c.WriteJSON(logFrame{Type: "error", Err: err.Error()})
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
			if err := c.WriteJSON(logFrame{Type: "line", Line: line}); err != nil {
				return
			}
		}
	}
}
