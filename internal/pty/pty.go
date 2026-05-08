// Package pty manages PTY-backed shell sessions used by the web terminal.
//
// Each Session pairs a child shell process with a PTY master file. Reads pull
// shell output; writes feed user input. Resize forwards window-size changes
// to the kernel. Close politely SIGHUPs the child, then SIGKILLs after a
// short grace period if it didn't exit.
//
// Security: shells are restricted to a small whitelist; the spawn inherits
// the controlroom service user (no setuid, no privilege escalation), and the
// environment is sanitized to a known-safe subset.
package pty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// AllowedShells lists the executables the terminal handler will spawn. Anything
// outside this set is rejected. Order doesn't matter.
var AllowedShells = []string{
	"/bin/bash",
	"/usr/bin/bash",
	"/bin/sh",
	"/usr/bin/sh",
	"/bin/zsh",
	"/usr/bin/zsh",
}

// PreferredShells is the fallback order if the client doesn't specify one.
var PreferredShells = []string{"/bin/bash", "/bin/sh"}

// MaxRows / MaxCols clamp window-size requests to sane bounds.
const (
	MaxRows = 500
	MaxCols = 500

	// CloseGrace is how long we wait between SIGHUP and SIGKILL during Close.
	CloseGrace = 5 * time.Second
)

// Session is one shell + its PTY master.
type Session struct {
	ID    string
	Shell string

	cmd  *exec.Cmd
	ptmx *os.File

	bytesIn  atomic.Uint64 // user → shell
	bytesOut atomic.Uint64 // shell → user

	startedAt time.Time

	closeOnce sync.Once
	closed    chan struct{}
}

// Options for New. Fields left zero/empty fall back to safe defaults.
type Options struct {
	Shell string // "" → first existing entry of PreferredShells
	Rows  int
	Cols  int
	Env   []string // appended to the sanitized base env
}

// New starts a shell under a fresh PTY. The returned Session is hot — the
// caller must Close() it eventually or the child leaks.
func New(id string, opts Options) (*Session, error) {
	shell, err := pickShell(opts.Shell)
	if err != nil {
		return nil, err
	}

	rows, cols := clampSize(opts.Rows, opts.Cols)

	cmd := exec.Command(shell)
	cmd.Env = baseEnv(opts.Env, shell)
	// Run in its own session so signals stay scoped.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}

	s := &Session{
		ID:        id,
		Shell:     shell,
		cmd:       cmd,
		ptmx:      ptmx,
		startedAt: time.Now(),
		closed:    make(chan struct{}),
	}
	return s, nil
}

// Read pulls the next chunk of shell output. Counts bytes for audit.
func (s *Session) Read(p []byte) (int, error) {
	n, err := s.ptmx.Read(p)
	if n > 0 {
		s.bytesOut.Add(uint64(n))
	}
	return n, err
}

// Write feeds user input into the shell.
func (s *Session) Write(p []byte) (int, error) {
	n, err := s.ptmx.Write(p)
	if n > 0 {
		s.bytesIn.Add(uint64(n))
	}
	return n, err
}

// Resize updates the kernel's idea of the window size so curses-style apps
// (top, vim, etc.) reflow correctly.
func (s *Session) Resize(rows, cols int) error {
	rows, cols = clampSize(rows, cols)
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// BytesIn / BytesOut snapshot the running counters.
func (s *Session) BytesIn() uint64  { return s.bytesIn.Load() }
func (s *Session) BytesOut() uint64 { return s.bytesOut.Load() }
func (s *Session) StartedAt() time.Time { return s.startedAt }

// Closed returns a channel that closes when the session has fully exited.
func (s *Session) Closed() <-chan struct{} { return s.closed }

// ExitCode reports the child's exit status; -1 if it hasn't exited yet or was
// signalled.
func (s *Session) ExitCode() int {
	if s.cmd.ProcessState == nil {
		return -1
	}
	return s.cmd.ProcessState.ExitCode()
}

// Close sends SIGHUP, waits CloseGrace for a clean exit, then SIGKILLs. Safe
// to call multiple times.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		defer close(s.closed)
		_ = s.ptmx.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(syscall.SIGHUP)
		}
		done := make(chan error, 1)
		go func() { done <- s.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(CloseGrace):
			if s.cmd.Process != nil {
				_ = s.cmd.Process.Kill()
			}
			<-done
		}
	})
	return nil
}

// ---- helpers ----

func pickShell(requested string) (string, error) {
	if requested != "" {
		if !isAllowed(requested) {
			return "", fmt.Errorf("shell %q not allowed", requested)
		}
		if _, err := os.Stat(requested); err != nil {
			return "", fmt.Errorf("shell %q not found on host", requested)
		}
		return requested, nil
	}
	for _, p := range PreferredShells {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no usable shell on host")
}

func isAllowed(path string) bool {
	for _, a := range AllowedShells {
		if a == path {
			return true
		}
	}
	return false
}

func clampSize(rows, cols int) (int, int) {
	if rows < 1 {
		rows = 24
	}
	if rows > MaxRows {
		rows = MaxRows
	}
	if cols < 1 {
		cols = 80
	}
	if cols > MaxCols {
		cols = MaxCols
	}
	return rows, cols
}

// baseEnv assembles a clean environment so the spawned shell doesn't inherit
// surprises from the systemd unit (e.g. CR_* vars containing tokens).
func baseEnv(extra []string, shell string) []string {
	allow := []string{"HOME", "USER", "LOGNAME", "PATH", "LANG", "LC_ALL", "TZ", "TERM"}
	out := make([]string, 0, len(allow)+len(extra)+2)
	for _, k := range allow {
		if v := os.Getenv(k); v != "" {
			out = append(out, k+"="+v)
		}
	}
	out = append(out, "SHELL="+shell)
	if !hasEnv(out, "TERM") {
		out = append(out, "TERM=xterm-256color")
	}
	for _, e := range extra {
		// Don't allow callers to overwrite SHELL or PATH.
		if strings.HasPrefix(e, "SHELL=") || strings.HasPrefix(e, "PATH=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

func hasEnv(env []string, key string) bool {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// CopyToWebSocket runs a read loop that copies shell output through the
// supplied callback. It returns when EOF is hit or the callback returns false
// (signalling the WS handler that the peer has gone). The callback is called
// from the read goroutine; it must not block indefinitely.
func (s *Session) CopyToWebSocket(write func([]byte) bool) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.Read(buf)
		if n > 0 {
			if !write(buf[:n]) {
				return nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				return nil
			}
			return err
		}
	}
}
