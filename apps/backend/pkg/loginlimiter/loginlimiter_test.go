package loginlimiter

import (
	"testing"
	"time"
)

func newTestLimiter(maxFailures int, window, lockout time.Duration) (*Limiter, *time.Time) {
	l := New(maxFailures, window, lockout)
	current := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return current }
	return l, &current
}

func TestLockoutAfterMaxFailures(t *testing.T) {
	l, _ := newTestLimiter(3, 15*time.Minute, 10*time.Minute)

	if locked := l.RecordFailure("k"); locked {
		t.Fatal("locked too early (1st failure)")
	}
	if locked := l.RecordFailure("k"); locked {
		t.Fatal("locked too early (2nd failure)")
	}
	if locked := l.RecordFailure("k"); !locked {
		t.Fatal("expected lockout on 3rd failure")
	}

	blocked, retry := l.Blocked("k")
	if !blocked {
		t.Fatal("expected key to be blocked")
	}
	if retry <= 0 || retry > 10*time.Minute {
		t.Fatalf("unexpected retry duration: %v", retry)
	}
}

func TestLockoutExpires(t *testing.T) {
	l, now := newTestLimiter(2, 15*time.Minute, 10*time.Minute)
	l.RecordFailure("k")
	l.RecordFailure("k")

	*now = now.Add(10*time.Minute + time.Second)
	if blocked, _ := l.Blocked("k"); blocked {
		t.Fatal("lockout should have expired")
	}
}

func TestWindowResetsFailureCount(t *testing.T) {
	l, now := newTestLimiter(3, 15*time.Minute, 10*time.Minute)
	l.RecordFailure("k")
	l.RecordFailure("k")

	*now = now.Add(16 * time.Minute)
	// New window: two more failures must not lock.
	l.RecordFailure("k")
	if locked := l.RecordFailure("k"); locked {
		t.Fatal("failures across windows must not accumulate")
	}
}

func TestResetClearsHistory(t *testing.T) {
	l, _ := newTestLimiter(2, 15*time.Minute, 10*time.Minute)
	l.RecordFailure("k")
	l.Reset("k")
	if locked := l.RecordFailure("k"); locked {
		t.Fatal("reset should clear failure history")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(2, 15*time.Minute, 10*time.Minute)
	l.RecordFailure("a")
	l.RecordFailure("a")
	if blocked, _ := l.Blocked("b"); blocked {
		t.Fatal("unrelated key must not be blocked")
	}
}

func TestPruneKeepsMapBounded(t *testing.T) {
	l, now := newTestLimiter(5, time.Minute, time.Minute)
	l.maxEntries = 10
	for i := 0; i < 10; i++ {
		l.RecordFailure(string(rune('a' + i)))
	}
	*now = now.Add(3 * time.Minute)
	l.RecordFailure("fresh")
	if len(l.entries) > 2 {
		t.Fatalf("expired entries not pruned, len=%d", len(l.entries))
	}
}
