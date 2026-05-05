# Phase 01 — Bot Skeleton

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §5.1 commands, §5.2 FSM
- Research: `./research/research-01-library-and-concurrency-decisions.md`
- Existing scaffold: `services/api/cmd/api/main.go` (extend to start bot goroutine)

## Overview
- **Priority:** P1 (blocks every other phase)
- **Status:** pending
- **Description:** Initialize `tgbotapi.BotAPI`, long-poll updates, dispatch to command router, Redis-backed FSM, middleware chain (auth/ban/admin/i18n).

## Key Insights
- Bot + API share the same `*pgxpool.Pool`, `*redis.Client`, `*zap.Logger` from `main.go`.
- Start bot in a goroutine launched alongside `app.Listen`; shutdown on same `SIGTERM` ctx.
- `tgbotapi` is sync per-update — we spawn worker goroutines bounded by `semaphore.NewWeighted(50)` to avoid goroutine explosion on ddos.
- `singleflight.Group` keyed on `telegram_id` ensures per-user updates serialize. Prevents double-tap race in `/start` trial grant.

## Requirements
### Functional
- Long-poll updates (offset persistence in Redis key `tg:poll_offset`).
- Route text commands `/cmd`, callback queries, contact shares.
- FSM read/write via Redis JSON.
- Graceful shutdown: stop receiving new updates, drain in-flight, timeout 10s.
- Panic recovery per update — log + reply generic error to user.

### Non-functional
- Per-user serialization (singleflight).
- Global concurrency cap 50 in-flight handlers.
- All handlers receive `context.Context` with 25s timeout.
- Log every update: `telegram_id`, `command`, `latency_ms`, `result`.

## Architecture
```
main.go
  ├─ app.Listen (Fiber HTTP)       — goroutine A
  └─ bot.Start(ctx, deps)          — goroutine B
        ├─ updates := api.GetUpdatesChan(cfg)
        └─ for u := range updates:
              sem.Acquire()
              go func() {
                  defer sem.Release()
                  singleflight.Do(key=tgID, fn=handle(u))
              }()

handle(u) → middleware chain → router → command handler
                  ├─ recover
                  ├─ logger
                  ├─ loadUser (creates if missing, idempotent)
                  ├─ banCheck (short-circuit if banned)
                  ├─ i18n (load language pref)
                  └─ route(command)
```

### FSM contract (`bot/state.go`)
```go
type State struct {
    Name      string          `json:"state"`    // "idle", "awaiting_contact", ...
    Data      json.RawMessage `json:"data"`     // per-state payload
    ExpiresAt int64           `json:"expires_at"`
}

func (s *Store) Load(ctx, tgID int64) (State, error)
func (s *Store) Save(ctx, tgID int64, state State, ttl time.Duration) error
func (s *Store) Clear(ctx, tgID int64) error
```

Redis key pattern: `tg:state:<telegram_id>`. TTL per-state: idle=no-key, awaiting_contact=30m, buy_*=30m, topup_waiting=24h, support_describing=30m.

## Related Code Files
### Create
- `services/api/internal/bot/bot.go` — `New(deps) *Bot`, `Start(ctx)`, `Stop()`
- `services/api/internal/bot/router.go` — `Route(ctx, update)` dispatcher
- `services/api/internal/bot/state.go` — FSM Redis store
- `services/api/internal/bot/middleware.go` — chain: recover/logger/loadUser/banCheck/i18n
- `services/api/internal/bot/deps.go` — `Deps` struct (pool, rdb, log, cfg, services...)

### Modify
- `services/api/cmd/api/main.go` — add bot goroutine + shutdown ordering
- `services/api/internal/config/config.go` — add `TelegramBotUsername` (for links in messages)
- `services/api/go.mod` — add `github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1`, `golang.org/x/sync` (already indirect — promote to direct)

## Implementation Steps
1. `go get github.com/go-telegram-bot-api/telegram-bot-api/v5`
2. Create `bot/deps.go` with `Deps` struct (embed pool, rdb, logger, cfg, nil-pointer services — services wired in later phases).
3. Create `bot/bot.go`:
   - `New(deps) (*Bot, error)` — calls `tgbotapi.NewBotAPI(token)`; if `TELEGRAM_BOT_TOKEN` empty, return `ErrBotDisabled` (non-fatal, API can run without bot in dev).
   - `Start(ctx)` — restores offset from Redis `tg:poll_offset`, loops `GetUpdates`, dispatches to `Route`.
   - `Stop()` — closes updates channel, waits for sem to drain (10s).
4. Create `bot/state.go` — FSM Load/Save/Clear on Redis, serialize via `encoding/json`.
5. Create `bot/middleware.go`:
   - `recover` → `defer recover() + log stack + reply "internal error, sorry"`
   - `logger` → start time, log at end with latency
   - `loadUser` → `SELECT ... FROM users WHERE telegram_id=$1`, create if missing (INSERT ON CONFLICT DO UPDATE RETURNING), attach to ctx
   - `banCheck` → if `is_banned`, reply "account disabled, contact @support" and short-circuit
   - `i18n` → set `lang` in ctx from `users.language`
6. Create `bot/router.go` — string switch on `update.Message.Command()` and `update.CallbackQuery.Data`. Unknown commands → send help message.
7. Wire in `main.go`:
   ```go
   bot, err := bot.New(deps)
   if err != nil && !errors.Is(err, bot.ErrBotDisabled) {
       log.Fatal(...)
   }
   if bot != nil {
       go bot.Start(ctx)
       defer bot.Stop()
   }
   ```
8. Add logging: `zap.String("component", "bot")` tagged logger.
9. Write `bot/state_test.go` with miniredis-backed assertion (load/save/clear/TTL).

## Todo List
- [ ] Add tgbotapi dependency; bump go.mod
- [ ] Create `bot/deps.go` with Deps struct
- [ ] Implement `bot/bot.go` with New/Start/Stop
- [ ] Implement `bot/state.go` Redis FSM
- [ ] Implement `bot/middleware.go` chain
- [ ] Implement `bot/router.go` dispatcher (command stubs)
- [ ] Modify `main.go` to launch bot goroutine
- [ ] Add `TELEGRAM_BOT_USERNAME` to config
- [ ] Write `state_test.go` with miniredis
- [ ] Manual smoke test: `/ping` command replies "pong"
- [ ] Verify `go build ./...` + `go vet ./...`

## Success Criteria
- `go build ./...` clean
- `TELEGRAM_BOT_TOKEN` empty → API boots, bot logs "disabled", no crash
- `TELEGRAM_BOT_TOKEN` set to test bot → `/ping` in Telegram replies "pong"
- Redis FSM test passes `-race`
- SIGTERM drains in-flight handlers within 10s

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Long-poll stalls on Telegram outage | Med | Low | tgbotapi handles retry; log errors, don't crash |
| Unbounded goroutines on update burst | Low | High | `semaphore.NewWeighted(50)` cap |
| FSM Redis key TTL drift | Low | Med | Always set TTL with SETEX; test `ExpiresAt` honored |
| Panic in handler kills bot | High | High | `recover` middleware first in chain |
| Offset loss on restart → reprocess old updates | Low | Low | Redis-persisted offset, update every batch |

## Security Considerations
- Never log bot token.
- Panic stacks logged to zap but redact any `Message.Text` beyond first 200 chars (may contain phone number from contact share).
- Ban check runs BEFORE any other logic — banned user cannot trigger side effects.

## Next Steps
- Phase 02 builds on `loadUser` middleware + state.Save for `awaiting_contact` flow.
- Phase 09 templates wired into middleware i18n context key.
