// Package logs serves /api/logs/journal and /ws/logs/journal.
package logs

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/logs"
)

type Deps struct {
	Logger zerolog.Logger
}

const (
	wsWriteWait = 5 * time.Second
	wsReadWait  = 70 * time.Second
	wsPing      = 30 * time.Second
)

func MountHTTP(authed fiber.Router, d Deps) {
	authed.Get("/logs/journal", d.queryHandler)
}

func MountWS(wsGroup fiber.Router, d Deps) {
	wsGroup.Get("/logs/journal", websocket.New(d.tailWS, websocket.Config{
		HandshakeTimeout: 5 * time.Second,
	}))
}

func filterFromQuery(c *fiber.Ctx) logs.Filter {
	priority := -1
	if p := c.Query("priority"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			priority = v
		}
	}
	n := 0
	if v := c.Query("n"); v != "" {
		if iv, err := strconv.Atoi(v); err == nil {
			n = iv
		}
	}
	return logs.Filter{
		Unit:     c.Query("unit"),
		Priority: priority,
		Since:    c.Query("since"),
		Until:    c.Query("until"),
		Search:   c.Query("q"),
		N:        n,
	}
}

type queryResp struct {
	Entries []logs.Entry `json:"entries"`
}

func (d Deps) queryHandler(c *fiber.Ctx) error {
	if !logs.Available() {
		return fiber.NewError(http.StatusServiceUnavailable, journalUnavailableMsg)
	}
	entries, err := logs.Query(c.Context(), filterFromQuery(c))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "journal query: "+err.Error())
	}
	return c.JSON(queryResp{Entries: entries})
}

// journalUnavailableMsg is shown to the operator when journalctl can't be
// invoked (e.g. distroless container). The SPA surfaces it verbatim.
const journalUnavailableMsg = "journal logs unavailable on this host (journalctl not found) — switch to Containers or run ControlRoom on bare metal"

// ---- live tail ----

// queryFrom is a tiny shim because we can't reuse fiber.Ctx here; we read
// query params from the upgraded WebSocket conn instead.
func queryFromConn(c *websocket.Conn) logs.Filter {
	q := c.Query
	priority := -1
	if v, err := strconv.Atoi(q("priority")); err == nil {
		priority = v
	}
	return logs.Filter{
		Unit:     q("unit"),
		Priority: priority,
		Since:    q("since"),
		Search:   q("q"),
	}
}

func (d Deps) tailWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	if !logs.Available() {
		_ = c.WriteJSON(fiber.Map{"type": "error", "err": journalUnavailableMsg})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = c.SetReadDeadline(time.Now().Add(wsReadWait))
	c.SetPongHandler(func(string) error { return c.SetReadDeadline(time.Now().Add(wsReadWait)) })

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	entries, err := logs.Tail(ctx, queryFromConn(c))
	if err != nil {
		_ = c.WriteJSON(fiber.Map{"type": "error", "err": err.Error()})
		return
	}

	ping := time.NewTicker(wsPing)
	defer ping.Stop()

	for {
		select {
		case <-closed:
			return
		case <-ping.C:
			_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case e, ok := <-entries:
			if !ok {
				return
			}
			_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.WriteJSON(e); err != nil {
				return
			}
		}
	}
}
