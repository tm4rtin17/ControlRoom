// Package systemd is a thin abstraction over the systemd D-Bus API used by
// the services dashboard.
//
// The Client interface intentionally exposes only the verbs ControlRoom needs
// (list / get / start / stop / restart / enable / disable) so a fake can be
// drop-in for tests and the dbus implementation can be swapped if needed.
package systemd

import (
	"context"
	"errors"
	"strings"
)

// Unit is the slim view returned by List — fast to fetch in bulk.
type Unit struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	LoadState     string `json:"load_state"`      // "loaded" | "error" | "masked" | "not-found"
	ActiveState   string `json:"active_state"`    // "active" | "inactive" | "failed" | "activating" | "deactivating"
	SubState      string `json:"sub_state"`       // "running", "dead", "exited", …
	UnitFileState string `json:"unit_file_state"` // "enabled" | "disabled" | "static" | "masked" | ""
}

// UnitDetail extends Unit with the more expensive properties we only fetch on
// inspection.
type UnitDetail struct {
	Unit
	FragmentPath  string   `json:"fragment_path"`
	Following     string   `json:"following"`
	MemoryCurrent uint64   `json:"memory_current"`
	TasksCurrent  uint64   `json:"tasks_current"`
	Triggers      []string `json:"triggers"`
	TriggeredBy   []string `json:"triggered_by"`
	Documentation []string `json:"documentation"`
}

// ListFilter filters the unit list. All fields combine with AND. The Search
// field is a case-insensitive substring match against name + description.
type ListFilter struct {
	// Suffixes restricts to specific unit types ("service", "socket", …).
	// Empty = all types.
	Suffixes []string
	Search   string
	// States restricts to active states. Empty = all.
	States []string
}

// Apply returns true if the unit passes the filter.
func (f ListFilter) Apply(u Unit) bool {
	if len(f.Suffixes) > 0 {
		ok := false
		for _, s := range f.Suffixes {
			if strings.HasSuffix(u.Name, "."+s) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(f.States) > 0 {
		ok := false
		for _, s := range f.States {
			if u.ActiveState == s {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if f.Search != "" {
		needle := strings.ToLower(f.Search)
		hay := strings.ToLower(u.Name + " " + u.Description)
		if !strings.Contains(hay, needle) {
			return false
		}
	}
	return true
}

// Client is what handlers depend on; the dbus implementation lives in dbus.go,
// the test fake in fake.go.
type Client interface {
	List(ctx context.Context, filter ListFilter) ([]Unit, error)
	Get(ctx context.Context, name string) (*UnitDetail, error)
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	Enable(ctx context.Context, name string) error
	Disable(ctx context.Context, name string) error
	Close() error
}

// ErrUnavailable signals that systemd is not reachable on this host. The
// /api/services endpoints return 503 when the configured Client is nil so
// dev environments without a system bus degrade gracefully.
var ErrUnavailable = errors.New("systemd is not available on this host")
