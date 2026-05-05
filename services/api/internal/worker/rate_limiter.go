// rate_limiter.go: Per-site token bucket ensuring minimum gap between posts to the same domain.
// Prevents hammering a single WordPress site when multiple jobs target it concurrently.
package worker

import (
	"context"
	"sync"
	"time"
)

const defaultMinGap = 30 * time.Second

// RateLimiter enforces a minimum time gap between consecutive posts to the same site domain.
// Uses a mutex-protected map instead of sync.Map because we need read-modify-write atomicity
// (check last time, compute wait, update last time) without a TOCTOU gap.
type RateLimiter struct {
	mu       sync.Mutex
	lastPost map[string]time.Time
	minGap   time.Duration
}

// NewRateLimiter creates a RateLimiter with the given minimum gap between posts per domain.
// Pass defaultMinGap (30s) for production use.
func NewRateLimiter(minGap time.Duration) *RateLimiter {
	if minGap <= 0 {
		minGap = defaultMinGap
	}
	return &RateLimiter{
		lastPost: make(map[string]time.Time),
		minGap:   minGap,
	}
}

// WaitForSlot blocks until it is safe to post to domain, then records the post time.
// Returns ctx.Err() if the context is cancelled while waiting.
//
// Algorithm:
//  1. Lock, read last post time for domain.
//  2. If elapsed < minGap, compute wait duration, unlock, sleep.
//  3. Re-lock after sleep, record current time, unlock.
func (r *RateLimiter) WaitForSlot(ctx context.Context, domain string) error {
	r.mu.Lock()
	last, ok := r.lastPost[domain]
	if ok {
		elapsed := time.Since(last)
		if elapsed < r.minGap {
			wait := r.minGap - elapsed
			r.mu.Unlock()
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
			r.mu.Lock()
		}
	}
	r.lastPost[domain] = time.Now()
	r.mu.Unlock()
	return nil
}
