// Package jobs is a tiny in-process runner for long-running shell commands.
//
// Each Run() call starts a goroutine that executes the supplied function and
// streams its output through a Job's ring buffer + listener channels. The
// runner is intentionally not durable — restarting the server cancels every
// in-flight job, which is acceptable for the homelab "apt upgrade" surface.
//
// A Subscribe() call returns the snapshot-so-far plus a live channel, so a
// SPA tab that reconnected mid-job sees the full output to-date and then
// continues live.
package jobs

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultBufferSize = 512 * 1024 // 512 KiB ring buffer per job
	listenerBuffer    = 32
)

// State is one of the lifecycle states a job can be in.
type State string

const (
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// Job represents one tracked command execution.
type Job struct {
	ID         string
	Action     string
	StartedAt  time.Time
	FinishedAt time.Time
	state      atomic.Value // State
	err        atomic.Value // error (or nil)

	mu        sync.Mutex
	output    *ringBuffer
	listeners map[uint64]chan []byte
	nextLis   uint64
	done      chan struct{}
	cancel    context.CancelFunc
}

func (j *Job) State() State {
	v, _ := j.state.Load().(State)
	if v == "" {
		return StateRunning
	}
	return v
}

func (j *Job) Error() error {
	v, _ := j.err.Load().(error)
	return v
}

// Done returns a channel that closes when the job has fully exited.
func (j *Job) Done() <-chan struct{} { return j.done }

// Snapshot returns the buffered output captured so far.
func (j *Job) Snapshot() []byte {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.output.Snapshot()
}

// Subscribe returns the snapshot of output so far and a channel that receives
// chunks of subsequent output. The unsubscribe func MUST be called or the
// listener leaks.
//
// If the job has already finished, snapshot is the full output and the
// returned channel is closed immediately.
func (j *Job) Subscribe() (snapshot []byte, ch <-chan []byte, unsubscribe func()) {
	j.mu.Lock()
	defer j.mu.Unlock()

	snapshot = j.output.Snapshot()

	if j.State() != StateRunning {
		closed := make(chan []byte)
		close(closed)
		return snapshot, closed, func() {}
	}

	id := j.nextLis
	j.nextLis++
	c := make(chan []byte, listenerBuffer)
	j.listeners[id] = c

	return snapshot, c, func() {
		j.mu.Lock()
		defer j.mu.Unlock()
		if existing, ok := j.listeners[id]; ok {
			delete(j.listeners, id)
			close(existing)
		}
	}
}

// Cancel signals the underlying context.
func (j *Job) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

// Write satisfies io.Writer; the runner sets cmd.Stdout/cmd.Stderr to the job
// itself so output is captured + fanned out.
func (j *Job) Write(p []byte) (int, error) {
	j.mu.Lock()
	j.output.Write(p)
	for id, c := range j.listeners {
		// Copy so listeners can't see later mutations.
		buf := make([]byte, len(p))
		copy(buf, p)
		select {
		case c <- buf:
		default:
			// Slow listener: drop it rather than block the runner.
			delete(j.listeners, id)
			close(c)
		}
	}
	j.mu.Unlock()
	return len(p), nil
}

// Runner is the registry of jobs.
type Runner struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func NewRunner() *Runner {
	return &Runner{jobs: make(map[string]*Job)}
}

// Run starts a new job. The supplied function gets a context (cancelled when
// Cancel() is called) and an io.Writer that feeds the job's output.
func (r *Runner) Run(parent context.Context, action string, fn func(ctx context.Context, w io.Writer) error) *Job {
	id := mintJobID()
	ctx, cancel := context.WithCancel(parent)
	j := &Job{
		ID:        id,
		Action:    action,
		StartedAt: time.Now(),
		output:    newRingBuffer(defaultBufferSize),
		listeners: make(map[uint64]chan []byte),
		done:      make(chan struct{}),
		cancel:    cancel,
	}
	j.state.Store(StateRunning)

	r.mu.Lock()
	r.jobs[id] = j
	r.mu.Unlock()

	go func() {
		defer cancel()
		err := fn(ctx, j)
		j.mu.Lock()
		j.FinishedAt = time.Now()
		switch {
		case errors.Is(err, context.Canceled):
			j.state.Store(StateCancelled)
		case err != nil:
			j.state.Store(StateFailed)
			j.err.Store(err)
		default:
			j.state.Store(StateSucceeded)
		}
		// Close any still-open listeners so SPA WS handlers exit.
		for id, c := range j.listeners {
			delete(j.listeners, id)
			close(c)
		}
		j.mu.Unlock()
		close(j.done)
	}()
	return j
}

// Get returns the job with the given id, or nil if unknown.
func (r *Runner) Get(id string) *Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jobs[id]
}

// ActiveByAction returns the running job matching action, or nil. Used to
// enforce single-active-upgrade semantics.
func (r *Runner) ActiveByAction(action string) *Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, j := range r.jobs {
		if j.Action == action && j.State() == StateRunning {
			return j
		}
	}
	return nil
}

// ---- ring buffer ----

type ringBuffer struct {
	buf bytes.Buffer
	cap int
}

func newRingBuffer(cap int) *ringBuffer {
	return &ringBuffer{cap: cap}
}

func (r *ringBuffer) Write(p []byte) {
	if len(p) >= r.cap {
		r.buf.Reset()
		_, _ = r.buf.Write(p[len(p)-r.cap:])
		return
	}
	if r.buf.Len()+len(p) > r.cap {
		drop := r.buf.Len() + len(p) - r.cap
		// Discard from the front by reading and re-buffering. For 512 KB this
		// is fine; if we ever push the cap up, switch to a real ring.
		current := r.buf.Bytes()
		r.buf.Reset()
		_, _ = r.buf.Write(current[drop:])
	}
	_, _ = r.buf.Write(p)
}

func (r *ringBuffer) Snapshot() []byte {
	out := make([]byte, r.buf.Len())
	copy(out, r.buf.Bytes())
	return out
}

func mintJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Discard satisfies io.Writer for tests.
var _ io.Writer = (*Job)(nil)
