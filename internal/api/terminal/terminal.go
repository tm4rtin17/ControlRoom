// Package terminal implements WS /ws/terminal: PTY over WebSocket.
//
// Wire protocol:
//   - Client → server (text frame): JSON control message.
//       First message: {"rows":N,"cols":M,"shell":"/bin/bash"}
//       Subsequent:    {"type":"resize","rows":N,"cols":M}
//   - Client → server (binary frame): raw input bytes piped to the PTY.
//   - Server → client (binary frame): raw PTY output.
//   - Server → client (text frame): {"type":"error","err":"..."} for fatal errors.
//
// Audit: a session_start row is written when the shell launches and a
// session_end row is written when it exits. Keystrokes are NEVER recorded —
// only byte counts.
package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	"github.com/tm4rtin17/controlroom/internal/pty"
	"github.com/tm4rtin17/controlroom/internal/store"
)

type Deps struct {
	DB     *store.DB
	Logger zerolog.Logger
	// IdleTimeout closes a session after this much inactivity (no read or
	// write). Zero falls back to defaultIdleTimeout.
	IdleTimeout time.Duration
	HostShell   bool // passed through from cfg.HostShell; wraps shell in nsenter
}

const (
	defaultIdleTimeout = 30 * time.Minute
	wsWriteWait        = 5 * time.Second
	wsReadWait         = 70 * time.Second
	wsPingInterval     = 30 * time.Second
)

func MountWS(wsGroup fiber.Router, d Deps) {
	wsGroup.Get("/terminal", websocket.New(d.handler, websocket.Config{
		HandshakeTimeout: 5 * time.Second,
	}))
}

type initMsg struct {
	Rows  int    `json:"rows"`
	Cols  int    `json:"cols"`
	Shell string `json:"shell,omitempty"`
}

type controlMsg struct {
	Type string `json:"type"`
	Rows int    `json:"rows,omitempty"`
	Cols int    `json:"cols,omitempty"`
}

type errorFrame struct {
	Type string `json:"type"`
	Err  string `json:"err"`
}

func (d Deps) handler(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	idle := d.IdleTimeout
	if idle <= 0 {
		idle = defaultIdleTimeout
	}

	_ = c.SetReadDeadline(time.Now().Add(wsReadWait))
	c.SetPongHandler(func(string) error { return c.SetReadDeadline(time.Now().Add(wsReadWait)) })

	// Read the initial negotiation frame (must be a text/JSON frame).
	mt, raw, err := c.ReadMessage()
	if err != nil {
		return
	}
	if mt != websocket.TextMessage {
		_ = c.WriteJSON(errorFrame{Type: "error", Err: "expected JSON init frame first"})
		return
	}

	var init initMsg
	if err := json.Unmarshal(raw, &init); err != nil {
		_ = c.WriteJSON(errorFrame{Type: "error", Err: "invalid init frame: " + err.Error()})
		return
	}

	sessionID := mintSessionID()
	sess, err := pty.New(sessionID, pty.Options{
		Shell:     init.Shell,
		Rows:      init.Rows,
		Cols:      init.Cols,
		HostShell: d.HostShell,
	})
	if err != nil {
		_ = c.WriteJSON(errorFrame{Type: "error", Err: err.Error()})
		return
	}
	defer sess.Close()

	// Audit start. The websocket.Conn has its own Locals() bridge to the
	// upstream Fiber context where RequireAuth attached the user.
	userID := int64(0)
	if u, ok := c.Locals(middleware.CtxUser).(*store.User); ok && u != nil {
		userID = u.ID
	}
	_ = d.DB.WriteAudit(context.Background(), store.AuditEntry{
		UserID:  userID,
		IP:      c.RemoteAddr().String(),
		Action:  "terminal.session_start",
		Target:  sessionID,
		Outcome: "success",
		Detail:  map[string]string{"shell": sess.Shell},
	})

	// Activity tracker for idle timeout.
	lastActivity := time.Now()
	activityCh := make(chan struct{}, 1)
	bumpActivity := func() {
		lastActivity = time.Now()
		select {
		case activityCh <- struct{}{}:
		default:
		}
	}

	// Goroutine 1: PTY → WS (binary frames).
	stopReader := make(chan struct{})
	go func() {
		defer close(stopReader)
		_ = sess.CopyToWebSocket(func(data []byte) bool {
			bumpActivity()
			_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return false
			}
			return true
		})
	}()

	// Goroutine 2: idle timer + pinger.
	stopIdle := make(chan struct{})
	go func() {
		defer close(stopIdle)
		ticker := time.NewTicker(15 * time.Second)
		ping := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		defer ping.Stop()
		for {
			select {
			case <-sess.Closed():
				return
			case <-activityCh:
				continue
			case <-ping.C:
				_ = c.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
					_ = sess.Close()
					return
				}
			case <-ticker.C:
				if time.Since(lastActivity) > idle {
					_ = c.WriteJSON(errorFrame{Type: "error", Err: "idle timeout"})
					_ = sess.Close()
					return
				}
			}
		}
	}()

	// Goroutine 3 (this one): WS → PTY.
	for {
		mt, data, err := c.ReadMessage()
		if err != nil {
			break
		}
		bumpActivity()
		switch mt {
		case websocket.BinaryMessage:
			if _, err := sess.Write(data); err != nil {
				goto done
			}
		case websocket.TextMessage:
			var ctrl controlMsg
			if err := json.Unmarshal(data, &ctrl); err != nil {
				continue
			}
			if ctrl.Type == "resize" {
				_ = sess.Resize(ctrl.Rows, ctrl.Cols)
			}
		}
	}
done:
	_ = sess.Close()
	<-stopReader
	<-stopIdle

	_ = d.DB.WriteAudit(context.Background(), store.AuditEntry{
		UserID:  userID,
		IP:      c.RemoteAddr().String(),
		Action:  "terminal.session_end",
		Target:  sessionID,
		Outcome: "success",
		Detail: map[string]any{
			"shell":       sess.Shell,
			"duration_ms": time.Since(sess.StartedAt()).Milliseconds(),
			"bytes_in":    sess.BytesIn(),
			"bytes_out":   sess.BytesOut(),
			"exit_code":   sess.ExitCode(),
		},
	})
}

func mintSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
