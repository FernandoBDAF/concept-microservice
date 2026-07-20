package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInMemoryGuard_Duplicate(t *testing.T) {
	g := NewInMemoryGuard()
	ctx := context.Background()
	key := KeyForAttempt("email-processing", "id-1", 0)

	if ok, err := g.Begin(ctx, key, time.Minute); err != nil || !ok {
		t.Fatalf("first Begin = (%v,%v), want (true,nil)", ok, err)
	}
	if ok, err := g.Begin(ctx, key, time.Minute); err != nil || ok {
		t.Fatalf("second Begin (duplicate) = (%v,%v), want (false,nil)", ok, err)
	}
}

func TestInMemoryGuard_ExpiryReleasesKey(t *testing.T) {
	g := NewInMemoryGuard()
	ctx := context.Background()
	key := KeyFor("q", "id-1")

	if ok, _ := g.Begin(ctx, key, time.Nanosecond); !ok {
		t.Fatal("first Begin should win")
	}
	time.Sleep(time.Millisecond) // let the TTL lapse
	if ok, _ := g.Begin(ctx, key, time.Minute); !ok {
		t.Error("after TTL expiry the key should be re-acquirable")
	}
}

// TestKeyForAttempt_ScopesRetries is the crux of the retry/idempotency
// interaction: a genuine duplicate (same attempt) collides and is deduped,
// while an intentional retry (incremented attempt) gets a distinct key and is
// reprocessed instead of being silently dropped.
func TestKeyForAttempt_ScopesRetries(t *testing.T) {
	base := KeyForAttempt("email-processing", "id-1", 0)
	if base != "idem:email-processing:id-1:0" {
		t.Errorf("KeyForAttempt shape = %q, want idem:email-processing:id-1:0", base)
	}
	if KeyForAttempt("email-processing", "id-1", 0) != base {
		t.Error("same attempt must produce the same key (duplicate dedupe)")
	}
	if KeyForAttempt("email-processing", "id-1", 1) == base {
		t.Error("a higher attempt must produce a different key (retry reprocessed)")
	}

	// End-to-end through the guard: dup of attempt 0 is blocked; attempt 1 wins.
	g := NewInMemoryGuard()
	ctx := context.Background()
	if ok, _ := g.Begin(ctx, KeyForAttempt("q", "id", 0), time.Hour); !ok {
		t.Fatal("attempt 0 first delivery should win")
	}
	if ok, _ := g.Begin(ctx, KeyForAttempt("q", "id", 0), time.Hour); ok {
		t.Error("attempt 0 redelivery should be deduped")
	}
	if ok, _ := g.Begin(ctx, KeyForAttempt("q", "id", 1), time.Hour); !ok {
		t.Error("attempt 1 (retry) should be admitted, not deduped")
	}
}

// fakeSetNXer drives the RedisGuard without a real Redis server.
type fakeSetNXer struct {
	ok  bool
	err error
}

func (f fakeSetNXer) SetNX(_ context.Context, _ string, _ any, _ time.Duration) (bool, error) {
	return f.ok, f.err
}

func TestRedisGuard_PropagatesResultAndError(t *testing.T) {
	ctx := context.Background()

	// Owns the key.
	if ok, err := NewRedisGuard(fakeSetNXer{ok: true}).Begin(ctx, "k", time.Minute); err != nil || !ok {
		t.Fatalf("Begin(success) = (%v,%v), want (true,nil)", ok, err)
	}

	// Duplicate: SETNX returns false.
	if ok, err := NewRedisGuard(fakeSetNXer{ok: false}).Begin(ctx, "k", time.Minute); err != nil || ok {
		t.Fatalf("Begin(dup) = (%v,%v), want (false,nil)", ok, err)
	}

	// Infra error is surfaced so BaseWorker can fail OPEN (process anyway).
	sentinel := errors.New("redis down")
	if ok, err := NewRedisGuard(fakeSetNXer{err: sentinel}).Begin(ctx, "k", time.Minute); !errors.Is(err, sentinel) || ok {
		t.Fatalf("Begin(err) = (%v,%v), want (false, redis down)", ok, err)
	}
}
