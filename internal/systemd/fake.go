package systemd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Fake is an in-memory Client used by tests and the future demo mode. Unsafe
// for concurrent use across goroutines without the lock — already taken
// internally for every method.
type Fake struct {
	mu    sync.Mutex
	units map[string]*UnitDetail
}

func NewFake(seed []UnitDetail) *Fake {
	f := &Fake{units: make(map[string]*UnitDetail, len(seed))}
	for i := range seed {
		u := seed[i]
		f.units[u.Name] = &u
	}
	return f
}

func (f *Fake) Close() error { return nil }

func (f *Fake) List(_ context.Context, filter ListFilter) ([]Unit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Unit, 0, len(f.units))
	for _, u := range f.units {
		if filter.Apply(u.Unit) {
			out = append(out, u.Unit)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *Fake) Get(_ context.Context, name string) (*UnitDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.units[name]
	if !ok {
		return nil, fmt.Errorf("unit %q not found", name)
	}
	cp := *u
	return &cp, nil
}

func (f *Fake) setActive(name, active, sub string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.units[name]
	if !ok {
		return errors.New("unit not found")
	}
	u.ActiveState = active
	u.SubState = sub
	return nil
}

func (f *Fake) Start(_ context.Context, name string) error {
	return f.setActive(name, "active", "running")
}

func (f *Fake) Stop(_ context.Context, name string) error {
	return f.setActive(name, "inactive", "dead")
}

func (f *Fake) Restart(_ context.Context, name string) error {
	return f.setActive(name, "active", "running")
}

func (f *Fake) Enable(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.units[name]
	if !ok {
		return errors.New("unit not found")
	}
	u.UnitFileState = "enabled"
	return nil
}

func (f *Fake) Disable(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.units[name]
	if !ok {
		return errors.New("unit not found")
	}
	u.UnitFileState = "disabled"
	return nil
}
