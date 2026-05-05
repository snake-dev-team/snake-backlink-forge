// Package bot — per-user mutex map, offset persistence, and GC helpers.
// Split from bot.go to keep both files under 200 lines.
package bot

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const (
	// userLockIdleEvict is how long a per-user lock entry lives after last use before GC.
	userLockIdleEvict = time.Hour
	// userLockGCInterval is how often the idle-lock GC goroutine runs.
	userLockGCInterval = 10 * time.Minute
	// offsetFlushInterval is how often maxOffset is flushed to Redis.
	offsetFlushInterval = time.Second
)

// perUserLock returns (or lazily creates) the serialization lock for tgID.
// The returned pointer is stable as long as the entry lives in the sync.Map.
// Concurrent calls with the same tgID always return the same pointer (LoadOrStore semantics).
func (b *Bot) perUserLock(tgID int64) *userLock {
	if lk, ok := b.userMu.Load(tgID); ok {
		u := lk.(*userLock)
		u.lastUsed.Store(time.Now().Unix())
		return u
	}
	u := &userLock{}
	u.lastUsed.Store(time.Now().Unix())
	actual, _ := b.userMu.LoadOrStore(tgID, u)
	return actual.(*userLock)
}

// persistOffset atomically records the max completed update offset using
// compare-and-swap so out-of-order goroutine completions always leave the
// highest offset in place.
func (b *Bot) persistOffset(next int64) {
	for {
		cur := b.maxOffset.Load()
		if next <= cur {
			return
		}
		if b.maxOffset.CompareAndSwap(cur, next) {
			return
		}
	}
}

// runOffsetFlusher ticks every offsetFlushInterval and writes maxOffset to Redis.
// Exits when loopCtx is cancelled; Stop() calls flushOffsetToRedis directly for
// the final flush after the drain window.
func (b *Bot) runOffsetFlusher() {
	ticker := time.NewTicker(offsetFlushInterval)
	defer ticker.Stop()

	var lastFlushed int64
	for {
		select {
		case <-b.loopCtx.Done():
			return
		case <-ticker.C:
			cur := b.maxOffset.Load()
			if cur > lastFlushed {
				b.flushOffsetToRedis()
				lastFlushed = cur
			}
		}
	}
}

// flushOffsetToRedis writes the current maxOffset to Redis.
// Uses a detached context.Background() with a 2s timeout so that shutdown
// cancellation of loopCtx/handlerCtx does not abort the write (H3 fix).
func (b *Bot) flushOffsetToRedis() {
	if b.deps.Rdb == nil {
		return
	}
	offset := b.maxOffset.Load()
	if offset == 0 {
		return
	}
	flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := b.deps.Rdb.Set(flushCtx, pollOffsetKey, strconv.FormatInt(offset, 10), 0).Err(); err != nil {
		b.deps.Log.Warn("bot: failed to persist poll offset", zap.Error(err))
	}
}

// runUserLockGC periodically evicts per-user lock entries idle for > userLockIdleEvict.
// This bounds sync.Map growth to recently-active users only.
func (b *Bot) runUserLockGC() {
	ticker := time.NewTicker(userLockGCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.loopCtx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-userLockIdleEvict).Unix()
			b.userMu.Range(func(k, v any) bool {
				lk := v.(*userLock)
				if lk.lastUsed.Load() < cutoff {
					b.userMu.Delete(k)
				}
				return true
			})
		}
	}
}
