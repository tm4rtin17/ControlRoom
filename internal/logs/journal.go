// Package logs wraps journalctl for the dashboard's log browser.
//
// We shell out rather than use the native sd-journal API: a dependency on
// libsystemd would force CGO, breaking our pure-Go static binary. journalctl
// JSON output is stable and small.
package logs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Entry is the SPA-facing shape of one journal line.
type Entry struct {
	Timestamp string `json:"timestamp"` // RFC 3339; converted from __REALTIME_TIMESTAMP
	Priority  int    `json:"priority"`  // 0 (emerg) .. 7 (debug)
	Unit      string `json:"unit,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	Message   string `json:"message"`
}

// Filter is the subset of journalctl flags the SPA can pass.
type Filter struct {
	Unit     string
	Priority int    // 0..7; pass -1 to skip
	Since    string // "-1h", "2025-05-08", "yesterday", …
	Until    string
	Search   string
	N        int // last-N entries (0 → no limit on tail mode; 200 default for static)
}

// Available reports whether `journalctl` is present in PATH. The result is
// cached for the lifetime of the process; we don't expect journalctl to come
// or go after boot.
//
// In container deployments (distroless image) this is false, and the API
// surface should return 503 rather than 500 with a confusing exec error.
func Available() bool { return availableOnce() }

var availableOnce = sync.OnceValue(func() bool {
	_, err := exec.LookPath("journalctl")
	return err == nil
})

// validUnit blocks shell injection through the unit name without locking us
// down to one filename pattern.
var validUnit = regexp.MustCompile(`^[a-zA-Z0-9@_.\-:\\]+\.(service|socket|target|timer|path|mount|slice|scope)$`)

// validSince covers "-1h", "2025-05-08 14:00:00", "yesterday", "today",
// "now". Anything else is rejected to keep journalctl from interpreting odd
// inputs or shell metacharacters (we never use a shell, but defense in depth).
var validSince = regexp.MustCompile(`^[a-zA-Z0-9 :\-]{1,32}$`)

// validSearch is the substring we pass to --grep. It must not contain regex
// anchors that could break the journalctl parser; we restrict to printable
// ASCII (sans regex metacharacters).
var validSearch = regexp.MustCompile(`^[a-zA-Z0-9 _\-./@:]{1,128}$`)

func (f Filter) args(follow bool) ([]string, error) {
	args := []string{"--output=json", "--no-pager"}
	if follow {
		args = append(args, "-f")
	}
	if f.Unit != "" {
		if !validUnit.MatchString(f.Unit) {
			return nil, fmt.Errorf("invalid unit name")
		}
		args = append(args, "-u", f.Unit)
	}
	if f.Priority >= 0 && f.Priority <= 7 {
		args = append(args, "-p", strconv.Itoa(f.Priority))
	}
	if f.Since != "" {
		if !validSince.MatchString(f.Since) {
			return nil, fmt.Errorf("invalid since")
		}
		args = append(args, "-S", f.Since)
	}
	if f.Until != "" {
		if !validSince.MatchString(f.Until) {
			return nil, fmt.Errorf("invalid until")
		}
		args = append(args, "-U", f.Until)
	}
	if f.Search != "" {
		if !validSearch.MatchString(f.Search) {
			return nil, fmt.Errorf("invalid search")
		}
		args = append(args, "--grep", f.Search)
	}
	if f.N > 0 {
		args = append(args, "-n", strconv.Itoa(f.N))
	} else if !follow {
		args = append(args, "-n", "200")
	}
	return args, nil
}

// Query runs a static journalctl query and returns the entries.
func Query(ctx context.Context, f Filter) ([]Entry, error) {
	args, err := f.args(false)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseEntries(out), nil
}

// Tail streams journal entries through the returned channel until ctx is
// cancelled. The child is killed (whole pgrp) when ctx fires.
func Tail(ctx context.Context, f Filter) (<-chan Entry, error) {
	args, err := f.args(true)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = io.Discard

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	ch := make(chan Entry, 64)

	go func() {
		defer close(ch)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)
		for sc.Scan() {
			e, ok := parseLine(sc.Bytes())
			if !ok {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case ch <- e:
			}
		}
	}()

	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = cmd.Wait()
	}()

	return ch, nil
}

// ---- parsing ----

func parseEntries(out []byte) []Entry {
	var entries []Entry
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)
	for sc.Scan() {
		if e, ok := parseLine(sc.Bytes()); ok {
			entries = append(entries, e)
		}
	}
	return entries
}

// rawJournal mirrors the small subset of journalctl --output=json fields
// we need. All fields arrive as strings; numeric ones are decoded by hand.
type rawJournal struct {
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
	Priority          string `json:"PRIORITY"`
	SystemdUnit       string `json:"_SYSTEMD_UNIT"`
	UserUnit          string `json:"_SYSTEMD_USER_UNIT"`
	Identifier        string `json:"SYSLOG_IDENTIFIER"`
	Comm              string `json:"_COMM"`
	PID               string `json:"_PID"`
	Hostname          string `json:"_HOSTNAME"`
	Message           any    `json:"MESSAGE"` // string or array of bytes
}

func parseLine(line []byte) (Entry, bool) {
	var raw rawJournal
	if err := json.Unmarshal(line, &raw); err != nil {
		return Entry{}, false
	}
	e := Entry{
		Unit:      coalesce(raw.SystemdUnit, raw.UserUnit),
		Identifier: coalesce(raw.Identifier, raw.Comm),
		Hostname:  raw.Hostname,
		Message:   stringOrBytes(raw.Message),
		Priority:  6,
	}
	if raw.Priority != "" {
		if p, err := strconv.Atoi(raw.Priority); err == nil {
			e.Priority = p
		}
	}
	if raw.PID != "" {
		if p, err := strconv.Atoi(raw.PID); err == nil {
			e.PID = p
		}
	}
	if raw.RealtimeTimestamp != "" {
		// Microseconds since epoch as a base-10 string.
		usec, err := strconv.ParseInt(raw.RealtimeTimestamp, 10, 64)
		if err == nil {
			sec := usec / 1_000_000
			ns := (usec % 1_000_000) * 1000
			e.Timestamp = formatRFC3339(sec, ns)
		}
	}
	return e, true
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// MESSAGE may be either a normal string or an array of bytes (when the
// underlying record had non-UTF8 data). We render the byte form as best we can.
func stringOrBytes(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		out := make([]byte, 0, len(x))
		for _, b := range x {
			if n, ok := b.(float64); ok {
				out = append(out, byte(int(n)))
			}
		}
		return string(out)
	default:
		return ""
	}
}

// formatRFC3339 returns RFC 3339 with nanosecond precision (microseconds
// padded with zeros).
func formatRFC3339(sec, ns int64) string {
	return time.Unix(sec, ns).UTC().Format(time.RFC3339Nano)
}

