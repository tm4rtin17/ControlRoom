// Package updates serves /api/updates/* and /ws/updates/jobs/:id.
package updates

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	"github.com/tm4rtin17/controlroom/internal/jobs"
	"github.com/tm4rtin17/controlroom/internal/store"
	"github.com/tm4rtin17/controlroom/internal/updates"
)

const (
	actionUpdate  = "updates.check"
	actionUpgrade = "updates.apply"
	actionReboot  = "system.reboot"

	wsWriteWait = 5 * time.Second
	wsReadWait  = 70 * time.Second
	wsPing      = 30 * time.Second
)

type Deps struct {
	Runner *jobs.Runner
	DB     *store.DB
	Logger zerolog.Logger
}

func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/updates")
	g.Get("/list", d.listHandler)
	g.Post("/check", d.checkHandler)
	g.Post("/apply", d.applyHandler)
	g.Get("/jobs/:id", d.jobStatusHandler)
}

func MountWS(wsGroup fiber.Router, d Deps) {
	wsGroup.Get("/updates/jobs/:id", websocket.New(d.jobStreamWS, websocket.Config{
		HandshakeTimeout: 5 * time.Second,
	}))
}

// ---- list ----

type listResp struct {
	Packages       []updates.Package `json:"packages"`
	RebootRequired bool              `json:"reboot_required"`
}

func (d Deps) listHandler(c *fiber.Ctx) error {
	pkgs, err := updates.Upgradable(c.Context())
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list upgradable: "+err.Error())
	}
	return c.JSON(listResp{Packages: pkgs, RebootRequired: updates.RebootRequired()})
}

// ---- check / apply ----

type jobResp struct {
	JobID string `json:"job_id"`
}

func (d Deps) checkHandler(c *fiber.Ctx) error {
	job := d.Runner.Run(c.UserContext(), actionUpdate, updates.RunUpdate)
	d.audit(c, "updates.check", job.ID, "started", nil)
	return c.JSON(jobResp{JobID: job.ID})
}

func (d Deps) applyHandler(c *fiber.Ctx) error {
	if active := d.Runner.ActiveByAction(actionUpgrade); active != nil {
		return c.Status(http.StatusConflict).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "conflict",
				"message": "an upgrade is already running",
				"detail":  fiber.Map{"job_id": active.ID},
			},
		})
	}
	job := d.Runner.Run(c.UserContext(), actionUpgrade, updates.RunUpgrade)
	d.audit(c, "updates.apply", job.ID, "started", nil)
	return c.JSON(jobResp{JobID: job.ID})
}

// ---- job status ----

type jobStatusResp struct {
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	State      string         `json:"state"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Error      string         `json:"error,omitempty"`
	Output     string         `json:"output"`
}

func (d Deps) jobStatusHandler(c *fiber.Ctx) error {
	job := d.Runner.Get(c.Params("id"))
	if job == nil {
		return fiber.NewError(http.StatusNotFound, "job not found")
	}
	return c.JSON(jobStatusToResp(job))
}

func jobStatusToResp(job *jobs.Job) jobStatusResp {
	resp := jobStatusResp{
		ID:        job.ID,
		Action:    job.Action,
		State:     string(job.State()),
		StartedAt: job.StartedAt,
		Output:    string(job.Snapshot()),
	}
	if !job.FinishedAt.IsZero() {
		t := job.FinishedAt
		resp.FinishedAt = &t
	}
	if e := job.Error(); e != nil {
		resp.Error = e.Error()
	}
	return resp
}

// ---- WS stream ----

type wsFrame struct {
	Type   string `json:"type"`           // "snapshot" | "chunk" | "state"
	Output string `json:"output,omitempty"`
	State  string `json:"state,omitempty"`
}

func (d Deps) jobStreamWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	job := d.Runner.Get(c.Params("id"))
	if job == nil {
		_ = c.WriteJSON(wsFrame{Type: "state", State: "not_found"})
		return
	}

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

	snapshot, ch, unsub := job.Subscribe()
	defer unsub()

	if len(snapshot) > 0 {
		_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
		if err := c.WriteJSON(wsFrame{Type: "snapshot", Output: string(snapshot)}); err != nil {
			return
		}
	}
	_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if err := c.WriteJSON(wsFrame{Type: "state", State: string(job.State())}); err != nil {
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
		case chunk, ok := <-ch:
			if !ok {
				// Job finished — send final state and exit.
				_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
				_ = c.WriteJSON(wsFrame{Type: "state", State: string(job.State())})
				return
			}
			_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.WriteJSON(wsFrame{Type: "chunk", Output: string(chunk)}); err != nil {
				return
			}
		case <-job.Done():
			_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
			_ = c.WriteJSON(wsFrame{Type: "state", State: string(job.State())})
			return
		}
	}
}

// ---- reboot ----

// MountReboot adds POST /api/system/reboot to the authenticated group.
// Lives in this package because the updates SPA is its main caller; M9 polish
// can move it to api/system if other surfaces need it.
func MountReboot(authed fiber.Router, d Deps) {
	authed.Post("/system/reboot", d.rebootHandler)
}

type rebootReq struct {
	Confirm string `json:"confirm"`
}

func (d Deps) rebootHandler(c *fiber.Ctx) error {
	var req rebootReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if req.Confirm != "REBOOT" {
		return fiber.NewError(http.StatusBadRequest, `body must include {"confirm":"REBOOT"}`)
	}

	job := d.Runner.Run(c.UserContext(), actionReboot, updates.RunReboot)
	d.audit(c, actionReboot, job.ID, "started", nil)
	return c.JSON(jobResp{JobID: job.ID})
}

// ---- helpers ----

func (d Deps) audit(c *fiber.Ctx, action, target, outcome string, detail any) {
	entry := store.AuditEntry{
		IP:      c.IP(),
		Action:  action,
		Target:  target,
		Outcome: outcome,
		Detail:  detail,
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}
	_ = d.DB.WriteAudit(c.Context(), entry)
}
