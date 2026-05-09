// Package system serves /api/system/overview and /ws/system/stats.
//
// Both endpoints share a single Aggregator (held in Deps), so multiple
// concurrent WS clients don't independently re-read /proc — the aggregator's
// 900ms cache turns the N×1Hz fanout into a single read per second.
package system

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/collectors"
	"github.com/tm4rtin17/controlroom/internal/docker"
	"github.com/tm4rtin17/controlroom/internal/k8s"
	"github.com/tm4rtin17/controlroom/internal/logs"
	"github.com/tm4rtin17/controlroom/internal/systemd"
)

type Deps struct {
	Aggregator *collectors.Aggregator
	Logger     zerolog.Logger
	// SystemD, Docker, and K8s are nil-or-set; mirror the same wiring used by
	// the services / containers / k8s handlers so /api/system/capabilities can
	// report which features the SPA should expose.
	SystemD systemd.Client
	Docker  docker.Client
	K8s     *k8s.Client
}

// MountHTTP registers the REST endpoint. Caller is responsible for applying
// auth + CSRF middleware.
func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/system")
	g.Get("/overview", d.overviewHandler)
	g.Get("/capabilities", d.capabilitiesHandler)
}

// capabilitiesResp tells the SPA which feature areas have a working backend
// on this host. The frontend uses it to hide nav entries that would otherwise
// dead-end with a 503 (e.g. the Services tab in container deployments).
type capabilitiesResp struct {
	Systemd    bool `json:"systemd"`
	Docker     bool `json:"docker"`
	Journal    bool `json:"journal"`
	Kubernetes bool `json:"kubernetes"`
}

func (d Deps) capabilitiesHandler(c *fiber.Ctx) error {
	return c.JSON(capabilitiesResp{
		Systemd:    d.SystemD != nil,
		Docker:     d.Docker != nil,
		Journal:    logs.Available(),
		Kubernetes: d.K8s != nil,
	})
}

// MountWS registers the WebSocket route. Caller applies auth (cookie-based)
// before this; CSRF is skipped because WS upgrade is a GET.
func MountWS(authed fiber.Router, d Deps) {
	g := authed.Group("/system")
	g.Get("/stats", websocket.New(d.statsWS, websocket.Config{
		HandshakeTimeout: 5 * time.Second,
	}))
}

func (d Deps) overviewHandler(c *fiber.Ctx) error {
	ov, errs := d.Aggregator.Snapshot()
	if len(errs) > 0 {
		d.Logger.Warn().Strs("errors", errs).Msg("partial system snapshot")
	}
	return c.Status(http.StatusOK).JSON(ov)
}

const (
	wsTickInterval = 1 * time.Second
	wsPingInterval = 30 * time.Second
	wsWriteWait    = 5 * time.Second
	wsReadWait     = 70 * time.Second // > pingInterval + grace
)

// statsWS pushes a snapshot every second; ping/pong every 30s keeps the
// connection alive through proxies. The connection closes on any write or
// read error (no reconnection logic here — the SPA reconnects).
func (d Deps) statsWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	c.SetReadDeadline(time.Now().Add(wsReadWait))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(wsReadWait))
	})

	tick := time.NewTicker(wsTickInterval)
	defer tick.Stop()
	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()

	// First snapshot immediately.
	if !d.send(c) {
		return
	}

	// Drain any client message in a goroutine so the read deadline keeps
	// advancing on pongs.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-closed:
			return
		case <-tick.C:
			if !d.send(c) {
				return
			}
		case <-ping.C:
			_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (d Deps) send(c *websocket.Conn) bool {
	ov, _ := d.Aggregator.Snapshot()
	_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if err := c.WriteJSON(ov); err != nil {
		return false
	}
	return true
}
