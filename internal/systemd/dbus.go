package systemd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

// DBusClient is the production Client implementation, talking to the system
// bus directly. Returned errors are typed as ErrUnavailable when the bus is
// unreachable so callers can decide between "503 Service Unavailable" and
// "500 Internal".
type DBusClient struct {
	conn *dbus.Conn
}

// NewDBus connects to the system bus. Returns ErrUnavailable if the host has
// no system bus (e.g. running in a stripped-down container).
func NewDBus(ctx context.Context) (*DBusClient, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	return &DBusClient{conn: conn}, nil
}

func (c *DBusClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.conn.Close()
	return nil
}

func (c *DBusClient) List(ctx context.Context, filter ListFilter) ([]Unit, error) {
	statuses, err := c.conn.ListUnitsContext(ctx)
	if err != nil {
		return nil, err
	}
	files, err := c.conn.ListUnitFilesContext(ctx)
	if err != nil {
		return nil, err
	}
	fileStates := make(map[string]string, len(files))
	for _, f := range files {
		// f.Path looks like "/lib/systemd/system/nginx.service".
		fileStates[filepath.Base(f.Path)] = f.Type
	}

	out := make([]Unit, 0, len(statuses))
	for _, s := range statuses {
		u := Unit{
			Name:          s.Name,
			Description:   s.Description,
			LoadState:     s.LoadState,
			ActiveState:   s.ActiveState,
			SubState:      s.SubState,
			UnitFileState: fileStates[s.Name],
		}
		if filter.Apply(u) {
			out = append(out, u)
		}
	}
	return out, nil
}

func (c *DBusClient) Get(ctx context.Context, name string) (*UnitDetail, error) {
	if !ValidUnitName(name) {
		return nil, fmt.Errorf("invalid unit name")
	}
	props, err := c.conn.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return nil, err
	}

	d := &UnitDetail{
		Unit: Unit{
			Name:          asString(props["Id"]),
			Description:   asString(props["Description"]),
			LoadState:     asString(props["LoadState"]),
			ActiveState:   asString(props["ActiveState"]),
			SubState:      asString(props["SubState"]),
			UnitFileState: asString(props["UnitFileState"]),
		},
		FragmentPath:  asString(props["FragmentPath"]),
		Following:     asString(props["Following"]),
		MemoryCurrent: cleanCounter(asUint64(props["MemoryCurrent"])),
		TasksCurrent:  cleanCounter(asUint64(props["TasksCurrent"])),
		Triggers:      asStringSlice(props["Triggers"]),
		TriggeredBy:   asStringSlice(props["TriggeredBy"]),
		Documentation: asStringSlice(props["Documentation"]),
	}
	return d, nil
}

func (c *DBusClient) Start(ctx context.Context, name string) error {
	return c.runJob(ctx, name, c.conn.StartUnitContext)
}

func (c *DBusClient) Stop(ctx context.Context, name string) error {
	return c.runJob(ctx, name, c.conn.StopUnitContext)
}

func (c *DBusClient) Restart(ctx context.Context, name string) error {
	return c.runJob(ctx, name, c.conn.RestartUnitContext)
}

type unitJobFn func(context.Context, string, string, chan<- string) (int, error)

func (c *DBusClient) runJob(ctx context.Context, name string, fn unitJobFn) error {
	if !ValidUnitName(name) {
		return errors.New("invalid unit name")
	}
	ch := make(chan string, 1)
	if _, err := fn(ctx, name, "replace", ch); err != nil {
		return err
	}
	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	select {
	case result := <-ch:
		if result != "done" {
			return fmt.Errorf("systemd job result: %s", result)
		}
		return nil
	case <-jobCtx.Done():
		return jobCtx.Err()
	}
}

func (c *DBusClient) Enable(ctx context.Context, name string) error {
	if !ValidUnitName(name) {
		return errors.New("invalid unit name")
	}
	if _, _, err := c.conn.EnableUnitFilesContext(ctx, []string{name}, false, true); err != nil {
		return err
	}
	return c.conn.ReloadContext(ctx)
}

func (c *DBusClient) Disable(ctx context.Context, name string) error {
	if !ValidUnitName(name) {
		return errors.New("invalid unit name")
	}
	if _, err := c.conn.DisableUnitFilesContext(ctx, []string{name}, false); err != nil {
		return err
	}
	return c.conn.ReloadContext(ctx)
}

// ---- property coercion helpers ----

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asUint64(v any) uint64 {
	switch x := v.(type) {
	case uint64:
		return x
	case int64:
		if x < 0 {
			return 0
		}
		return uint64(x)
	default:
		return 0
	}
}

func asStringSlice(v any) []string {
	if ss, ok := v.([]string); ok {
		return ss
	}
	return nil
}

// cleanCounter normalizes systemd's "unset" sentinel (max uint64) to 0 so the
// SPA can render "—" instead of an absurd value.
func cleanCounter(v uint64) uint64 {
	if v == ^uint64(0) {
		return 0
	}
	return v
}
