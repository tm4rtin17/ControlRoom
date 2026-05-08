package auth

import (
	"math"
	"sync"
	"time"
)

// IPLimiter is a per-key token bucket. Used for /api/auth/login at 5/min/IP.
type IPLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*ipBucket
	capacity  float64
	refillPS  float64 // tokens per second
	expireDur time.Duration
}

type ipBucket struct {
	tokens   float64
	lastFill time.Time
	touched  time.Time
}

// NewIPLimiter — capacity 5, 1 token / 12s ≈ 5 / minute.
func NewIPLimiter(perMinute int) *IPLimiter {
	cap := float64(perMinute)
	if cap < 1 {
		cap = 1
	}
	return &IPLimiter{
		buckets:   make(map[string]*ipBucket),
		capacity:  cap,
		refillPS:  cap / 60.0,
		expireDur: 10 * time.Minute,
	}
}

// Allow consumes one token if available. Returns (true, 0) on allow, or
// (false, retryAfter) on deny.
func (l *IPLimiter) Allow(key string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.expireLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &ipBucket{tokens: l.capacity, lastFill: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens = math.Min(l.capacity, b.tokens+elapsed*l.refillPS)
	b.lastFill = now
	b.touched = now

	if b.tokens < 1 {
		// time until tokens >= 1
		need := 1 - b.tokens
		retry := time.Duration(need/l.refillPS*float64(time.Second)) + time.Second
		return false, retry
	}
	b.tokens--
	return true, 0
}

func (l *IPLimiter) expireLocked(now time.Time) {
	if len(l.buckets) < 1024 {
		return
	}
	for k, b := range l.buckets {
		if now.Sub(b.touched) > l.expireDur {
			delete(l.buckets, k)
		}
	}
}

// UserBackoff implements per-username exponential backoff: 30s, 60s, 2m, 4m,
// …, capped at 1h. Resets on successful login.
type UserBackoff struct {
	mu    sync.Mutex
	state map[string]*backoff
}

type backoff struct {
	failures  int
	nextAllow time.Time
	touched   time.Time
}

func NewUserBackoff() *UserBackoff {
	return &UserBackoff{state: make(map[string]*backoff)}
}

const (
	backoffBase = 30 * time.Second
	backoffMax  = 1 * time.Hour
)

func (b *UserBackoff) CanAttempt(username string) (bool, time.Duration) {
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.state[username]
	if !ok || now.After(st.nextAllow) {
		return true, 0
	}
	return false, st.nextAllow.Sub(now)
}

func (b *UserBackoff) Failure(username string) time.Duration {
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.state[username]
	if !ok {
		st = &backoff{}
		b.state[username] = st
	}
	st.failures++
	delay := backoffBase << minInt(st.failures-1, 8) // 30s, 60s, 2m, ..., capped via min
	if delay > backoffMax {
		delay = backoffMax
	}
	st.nextAllow = now.Add(delay)
	st.touched = now
	return delay
}

func (b *UserBackoff) Success(username string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.state, username)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
