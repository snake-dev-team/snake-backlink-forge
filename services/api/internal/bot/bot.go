// Package bot implements the Telegram long-poll bot for Snake Backlink Forge.
// It shares the process's pgxpool, redis client, and zap logger from main.go.
package bot

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// ErrBotDisabled is returned by New when TELEGRAM_BOT_TOKEN is empty.
// Callers should treat this as non-fatal — the API runs without the bot in dev.
var ErrBotDisabled = errors.New("bot: TELEGRAM_BOT_TOKEN not configured")

const (
	// maxConcurrent caps simultaneous in-flight update handlers.
	maxConcurrent = 50
	// handlerTimeout is the per-update context deadline.
	handlerTimeout = 25 * time.Second
	// shutdownDrainTimeout is how long Stop waits for in-flight handlers to finish.
	shutdownDrainTimeout = 10 * time.Second
	// pollOffsetKey is the Redis key for persisting the getUpdates offset across restarts.
	pollOffsetKey = "tg:poll_offset"
)

// userLock is a per-Telegram-user mutex for serializing concurrent updates.
// lastUsed enables time-based eviction from the userMu map.
type userLock struct {
	mu       sync.Mutex
	lastUsed atomic.Int64 // unix seconds
}

// Bot wraps a tgbotapi.BotAPI with concurrency controls and lifecycle management.
// loopCtx stops new updates; handlerCtx (cancelled after drain) lets in-flights finish.
type Bot struct {
	api           *tgbotapi.BotAPI
	deps          *Deps
	sem           *semaphore.Weighted
	stopCh        chan struct{}
	loopCtx       context.Context
	loopCancel    context.CancelFunc
	handlerCtx    context.Context
	handlerCancel context.CancelFunc
	userMu        sync.Map   // map[int64]*userLock — per-user serialization; see bot_concurrency_helpers.go
	maxOffset     atomic.Int64 // max completed (updateID+1); flushed to Redis by runOffsetFlusher
}

// New creates a Bot from the given Deps.
// Returns ErrBotDisabled when cfg.TelegramBotToken is empty (non-fatal for callers).
func New(deps *Deps) (*Bot, error) {
	if deps.Cfg.TelegramBotToken == "" {
		return nil, ErrBotDisabled
	}

	api, err := tgbotapi.NewBotAPI(deps.Cfg.TelegramBotToken)
	if err != nil {
		return nil, fmt.Errorf("bot: init api: %w", err)
	}

	deps.Log.Info("bot authenticated", zap.String("username", api.Self.UserName))

	// Publish command list so Telegram's "Menu" button shows /start, /balance, etc.
	// Best-effort: failure is logged but doesn't block startup.
	registerCommandMenu(api, deps.Log)

	loopCtx, loopCancel := context.WithCancel(context.Background())
	handlerCtx, handlerCancel := context.WithCancel(context.Background())

	return &Bot{
		api:           api,
		deps:          deps,
		sem:           semaphore.NewWeighted(maxConcurrent),
		stopCh:        make(chan struct{}),
		loopCtx:       loopCtx,
		loopCancel:    loopCancel,
		handlerCtx:    handlerCtx,
		handlerCancel: handlerCancel,
	}, nil
}

// Start begins long-polling Telegram for updates and dispatching them to handlers.
// Blocks until Stop() is called. The ctx param is unused (Bot owns its own contexts).
func (b *Bot) Start(_ context.Context) {
	log := b.deps.Log

	// Restore poll offset from Redis to avoid reprocessing updates after restart.
	offset := b.loadOffset(b.loopCtx)

	ucfg := tgbotapi.NewUpdate(offset)
	ucfg.Timeout = 30

	updates := b.api.GetUpdatesChan(ucfg)

	log.Info("bot polling started", zap.Int("offset", offset))

	go b.runOffsetFlusher() // see bot_concurrency_helpers.go
	go b.runUserLockGC()

	for {
		select {
		case <-b.loopCtx.Done():
			log.Info("bot loop ctx done — stopping poll loop")
			return
		case <-b.stopCh:
			log.Info("bot stop signal — stopping poll loop")
			return
		case update, ok := <-updates:
			if !ok {
				log.Info("updates channel closed")
				return
			}
			if err := b.sem.Acquire(b.loopCtx, 1); err != nil {
				return
			}
			u := update
			go func() {
				defer b.sem.Release(1)
				b.handleUpdate(b.handlerCtx, u)
				b.persistOffset(int64(u.UpdateID + 1)) // C2: after handler, not before
			}()
		}
	}
}

// Stop signals the poll loop to exit and waits for in-flight handlers to drain
// (up to shutdownDrainTimeout). Safe to call multiple times.
func (b *Bot) Stop() {
	select {
	case <-b.stopCh:
		// Already closed — nothing to do.
	default:
		close(b.stopCh)
	}

	// 1. Stop the poll loop from dispatching new updates.
	b.loopCancel()

	// 2. Drain: acquire all slots — succeeds only when every handler has released.
	drainCtx, cancel := context.WithTimeout(context.Background(), shutdownDrainTimeout)
	defer cancel()

	if err := b.sem.Acquire(drainCtx, maxConcurrent); err != nil {
		b.deps.Log.Warn("bot shutdown drain timed out; some handlers may still be running",
			zap.Error(err))
	} else {
		b.sem.Release(maxConcurrent)
	}

	// 3. Cancel handler parent ctx AFTER drain window (C3 fix).
	b.handlerCancel()

	// 4. Flush final offset to Redis with a detached ctx (H3 fix).
	b.flushOffsetToRedis()

	b.deps.Log.Info("bot shutdown complete")
}

// handleUpdate dispatches a single update through the middleware chain.
// Per-user serialization via perUserLock ensures concurrent updates for the same user
// run sequentially — the second update waits for the first to complete (not dropped).
func (b *Bot) handleUpdate(parentCtx context.Context, update tgbotapi.Update) {
	tgID := updateTelegramID(update)

	lk := b.perUserLock(tgID)
	lk.mu.Lock()
	defer lk.mu.Unlock()

	ctx, cancel := context.WithTimeout(parentCtx, handlerTimeout)
	defer cancel()

	handler := buildChain(b.deps, route(b))
	if err := handler(ctx, b.api, update); err != nil {
		b.deps.Log.Warn("update handler returned error",
			zap.Int64("tg_id", tgID),
			zap.Error(err),
		)
	}
}

// Api returns the underlying tgbotapi.BotAPI instance.
// Used by main.go to pass the bot API to ConsumeAdminAlerts without exposing
// the full Bot struct to the notify consumer goroutine.
func (b *Bot) Api() *tgbotapi.BotAPI {
	return b.api
}

// loadOffset reads the persisted poll offset from Redis.
// Returns 0 on any error (safe default — Telegram deduplicates old updates).
func (b *Bot) loadOffset(ctx context.Context) int {
	if b.deps.Rdb == nil {
		return 0
	}
	val, err := b.deps.Rdb.Get(ctx, pollOffsetKey).Int()
	if err != nil {
		return 0
	}
	return val
}
