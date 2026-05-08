package docker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Fake is an in-memory Client used in tests. It does not try to simulate the
// log/stats streams — those return empty channels — but it does maintain
// container lifecycle state so handler tests can exercise Start/Stop/Remove.
type Fake struct {
	mu         sync.Mutex
	containers map[string]*ContainerDetail
}

func NewFake(seed []ContainerDetail) *Fake {
	f := &Fake{containers: make(map[string]*ContainerDetail, len(seed))}
	for i := range seed {
		c := seed[i]
		f.containers[c.ID] = &c
	}
	return f
}

func (f *Fake) Close() error { return nil }

func (f *Fake) List(_ context.Context, opts ListOptions) ([]Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Container, 0, len(f.containers))
	for _, c := range f.containers {
		if !opts.All && c.State != "running" {
			continue
		}
		out = append(out, c.Container)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *Fake) Inspect(_ context.Context, id string) (*ContainerDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.containers[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	cp := *c
	return &cp, nil
}

func (f *Fake) Start(_ context.Context, id string) error {
	return f.transition(id, "running", time.Now(), time.Time{})
}

func (f *Fake) Stop(_ context.Context, id string, _ time.Duration) error {
	return f.transition(id, "exited", time.Time{}, time.Now())
}

func (f *Fake) Restart(_ context.Context, id string, _ time.Duration) error {
	return f.transition(id, "running", time.Now(), time.Time{})
}

func (f *Fake) Remove(_ context.Context, id string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.containers[id]; !ok {
		return errors.New("not found")
	}
	delete(f.containers, id)
	return nil
}

func (f *Fake) Logs(_ context.Context, _ string, _ int) (<-chan LogLine, error) {
	ch := make(chan LogLine)
	close(ch)
	return ch, nil
}

func (f *Fake) Stats(_ context.Context, _ string) (<-chan ContainerStats, error) {
	ch := make(chan ContainerStats)
	close(ch)
	return ch, nil
}

func (f *Fake) transition(id, state string, started, finished time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.containers[id]
	if !ok {
		return errors.New("not found")
	}
	c.State = state
	c.Container.State = state
	if !started.IsZero() {
		c.StartedAt = started
		c.FinishedAt = time.Time{}
	}
	if !finished.IsZero() {
		c.FinishedAt = finished
	}
	return nil
}
