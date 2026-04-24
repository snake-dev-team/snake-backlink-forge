// cmd_regenkey_ratelimit_test.go — unit tests for regenkey rate-limit functions.
// Tests exercise peekRegenRateLimit and incrRegenRateLimit directly using miniredis.
// No tgbotapi mock overhead: we test the rate-limit layer in isolation (M1 fix).
package bot

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// newTestRdb spins up a miniredis server and returns a go-redis client + cleanup.
func newTestRdb(t *testing.T) (*goredis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// makeDeps builds a minimal Deps with only Rdb populated for rate-limit tests.
func makeDepsRdb(rdb *goredis.Client) *Deps {
	return &Deps{Rdb: rdb}
}

// TestRegenRateLimit_FourthCallBlocked verifies:
//  1. Three successful Issue simulations (manual INCR) raise counter to 3.
//  2. Fourth peek returns limited=true before any Issue is called.
//  3. Redis key TTL is ~86400s.
//  4. After FastForward past 24h, peek returns limited=false (counter expired).
func TestRegenRateLimit_FourthCallBlocked(t *testing.T) {
	rdb, mr := newTestRdb(t)
	deps := makeDepsRdb(rdb)
	ctx := context.Background()

	userID := "test-user-ratelimit-uuid"

	// Calls 1-3: peek (not limited) then INCR (simulating successful issuance).
	for i := 1; i <= 3; i++ {
		limited, err := peekRegenRateLimit(ctx, deps, userID)
		if err != nil {
			t.Fatalf("call %d: peekRegenRateLimit error: %v", i, err)
		}
		if limited {
			t.Fatalf("call %d: expected not limited before INCR, got limited=true", i)
		}

		if err = incrRegenRateLimit(ctx, deps, userID); err != nil {
			t.Fatalf("call %d: incrRegenRateLimit error: %v", i, err)
		}
	}

	// Verify counter = 3.
	count, err := rdb.Get(ctx, regenRLKey(userID)).Int()
	if err != nil {
		t.Fatalf("GET counter: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected counter=3 after 3 INCRs, got %d", count)
	}

	// Call 4: peek must return limited=true — Issue must NOT be called.
	limited, err := peekRegenRateLimit(ctx, deps, userID)
	if err != nil {
		t.Fatalf("call 4: peekRegenRateLimit error: %v", err)
	}
	if !limited {
		t.Fatal("call 4: expected limited=true, got false (4th rotation should be blocked)")
	}

	// Verify Redis key TTL is approximately 86400s (±60s tolerance for miniredis).
	ttl := mr.TTL(regenRLKey(userID))
	if ttl < 23*time.Hour || ttl > 25*time.Hour {
		t.Errorf("expected TTL ~86400s, got %v", ttl)
	}

	// Fast-forward past 24h — counter expires, next peek should be allowed.
	mr.FastForward(25 * time.Hour)

	limited, err = peekRegenRateLimit(ctx, deps, userID)
	if err != nil {
		t.Fatalf("post-expiry peekRegenRateLimit error: %v", err)
	}
	if limited {
		t.Fatal("after TTL expiry, expected limited=false (counter reset), got true")
	}
}

// TestRegenRateLimit_PeekDoesNotIncrement verifies peekRegenRateLimit never mutates Redis.
func TestRegenRateLimit_PeekDoesNotIncrement(t *testing.T) {
	rdb, _ := newTestRdb(t)
	deps := makeDepsRdb(rdb)
	ctx := context.Background()

	userID := "test-user-peek-uuid"

	// Call peek 10 times — counter must remain absent (no key created).
	for i := 0; i < 10; i++ {
		limited, err := peekRegenRateLimit(ctx, deps, userID)
		if err != nil {
			t.Fatalf("iteration %d: peekRegenRateLimit error: %v", i, err)
		}
		if limited {
			t.Fatalf("iteration %d: peek should never be limited when no INCR happened", i)
		}
	}

	exists, err := rdb.Exists(ctx, regenRLKey(userID)).Result()
	if err != nil {
		t.Fatalf("EXISTS check: %v", err)
	}
	if exists != 0 {
		t.Fatal("peek must not create Redis key; key exists after 10 peeks")
	}
}

// TestRegenRateLimit_KeyNaming verifies the Redis key uses the new searchable prefix.
func TestRegenRateLimit_KeyNaming(t *testing.T) {
	key := regenRLKey("some-uuid")
	expected := "regenkey_rate_limit:some-uuid"
	if key != expected {
		t.Fatalf("regenRLKey = %q, want %q", key, expected)
	}
}
