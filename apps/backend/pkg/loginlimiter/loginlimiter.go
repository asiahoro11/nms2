// Package loginlimiter provides an in-memory failed-attempt rate limiter for
// authentication endpoints. It is intentionally process-local: this server
// runs as a single instance, so no shared store is needed.
package loginlimiter

import (
	"sync"
	"time"
)

type entry struct {
	failures    int
	windowEnd   time.Time
	lockedUntil time.Time
}

// Limiter tracks failed attempts per key and locks a key out after too many
// failures inside the counting window.
type Limiter struct {
	mu          sync.Mutex
	entries     map[string]*entry
	maxFailures int
	window      time.Duration
	lockout     time.Duration
	maxEntries  int
	now         func() time.Time
}

// New creates a limiter that locks a key for `lockout` after `maxFailures`
// failures within `window`.
func New(maxFailures int, window, lockout time.Duration) *Limiter {
	return &Limiter{
		entries:     make(map[string]*entry),
		maxFailures: maxFailures,
		window:      window,
		lockout:     lockout,
		maxEntries:  10000,
		now:         time.Now,
	}
}

// Blocked reports whether the key is currently locked out and, if so, for how
// much longer.
func (l *Limiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok {
		return false, 0
	}
	now := l.now()
	if now.Before(e.lockedUntil) {
		return true, e.lockedUntil.Sub(now)
	}
	if now.After(e.windowEnd) {
		delete(l.entries, key)
	}
	return false, 0
}

// RecordFailure counts one failed attempt for the key. It returns true when
// this failure caused (or extended) a lockout.
func (l *Limiter) RecordFailure(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	e, ok := l.entries[key]
	if !ok || now.After(e.windowEnd) {
		if !ok && len(l.entries) >= l.maxEntries {
			l.pruneExpiredLocked(now)
		}
		e = &entry{windowEnd: now.Add(l.window)}
		l.entries[key] = e
	}

	e.failures++
	if e.failures >= l.maxFailures {
		e.lockedUntil = now.Add(l.lockout)
		// Keep the entry alive at least as long as the lockout.
		if e.windowEnd.Before(e.lockedUntil) {
			e.windowEnd = e.lockedUntil
		}
		return true
	}
	return false
}

// Reset clears the failure history for a key (call on successful auth).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// pruneExpiredLocked drops entries whose window and lockout have both passed.
// Caller must hold l.mu.
func (l *Limiter) pruneExpiredLocked(now time.Time) {
	for k, e := range l.entries {
		if now.After(e.windowEnd) && now.After(e.lockedUntil) {
			delete(l.entries, k)
		}
	}
}
