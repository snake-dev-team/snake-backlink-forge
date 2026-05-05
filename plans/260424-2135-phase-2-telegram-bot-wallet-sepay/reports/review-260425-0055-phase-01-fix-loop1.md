# Code Review — Phase 01 Bot Skeleton (fix loop 1 verify)

## Verdict
APPROVED

## Summary
All 3 critical + 3 high findings from the prior review closed cleanly. Per-user mutex with time-based GC replaces singleflight; offset saved after handler with CAS-max semantic; loop/handler contexts decoupled; loadUser fail-closed; nil-guards added on CallbackQuery.Message; flush uses detached ctx. Build + vet + 11/11 tests pass. No new critical/high regressions introduced. Commit-ready.

## Prior findings closure

| ID | Claim | Status | File:line |
|---|---|---|---|
| C1 | singleflight removed, per-user mutex | CLOSED | `bot.go:51,167-169` + `bot_concurrency_helpers.go:25-35` |
| C2 | offset after handler + max CAS | CLOSED | `bot.go:122` + `bot_concurrency_helpers.go:40-50` |
| C3 | loopCtx/handlerCtx decouple | CLOSED | `bot.go:47-50,69-70,139-153` + `main.go:116-128` |
| H1 | loadUser fail-closed | CLOSED | `middleware.go:123-131` |
| H2 | Message nil-check | CLOSED | `update_helpers.go:64-65` + `router.go:83-85` |
| H3 | detached ctx flush | CLOSED | `bot_concurrency_helpers.go:77-90` |
| M1 | banCheck deps drop | CLOSED | `middleware.go:141` |
| M2 | TelegramID field drop | CLOSED | `update_helpers.go:23-27` |
| L1 | singleflight comment update | CLOSED | no "singleflight" string anywhere in `services/api/` |

### C1 verification — per-user mutex
- `Grep "singleflight"` → 0 matches across `services/api/` (dep dropped from callers; `x/sync` still in go.mod for `semaphore` only).
- `perUserLock` (bot_concurrency_helpers.go:25-35) uses `sync.Map.LoadOrStore` — stable pointer, correct LoadOrStore semantics.
- `handleUpdate` (bot.go:167-169): `lk.mu.Lock(); defer lk.mu.Unlock()` — canonical pattern, no leak.
- `TestPerUserLock_SerializesConcurrentUpdates` (bot_per_user_lock_test.go:41-90): captures A's end + B's start as atomic unix-nanos, asserts `startB >= endA`. Genuinely proves serialization — not flaky (60ms hold, 5ms pre-sleep ensures A acquires first; if B ever raced through without A releasing, startB would be before endA by a measurable margin).
- `TestPerUserLock_DifferentUsersRunConcurrently` (line 94-137) proves isolation between distinct tgIDs.
- GC goroutine (`runUserLockGC`, line 94-113) ticks every 10min (`userLockGCInterval`), evicts entries with `lastUsed < now - 1h`. Exits on `loopCtx.Done()`. Bounded growth confirmed.

### C2 verification — offset after handler + CAS max
- bot.go:122: `b.persistOffset(int64(u.UpdateID + 1))` now runs AFTER `b.handleUpdate` returns, inside the same goroutine.
- `persistOffset` (bot_concurrency_helpers.go:40-50) uses CAS loop on `atomic.Int64`. If two goroutines complete out of order, the higher offset always wins.
- `runOffsetFlusher` ticker (1s interval) flushes to Redis only when `cur > lastFlushed` — avoids redundant writes.
- `loadOffset` (bot.go:185-194) restores on Start → no reprocessing of already-handled updates.
- `TestPersistOffset_MaxSemantics` (bot_per_user_lock_test.go:141-152) asserts: call 5,3,7,6 → result 7. Covers out-of-order.
- Stop() flushes final offset via `flushOffsetToRedis` AFTER drain, before return (bot.go:156).

### C3 verification — loopCtx/handlerCtx decoupled
- Bot struct (bot.go:42-53) holds both contexts with cancel funcs.
- `New()` (bot.go:69-70) initializes each via `context.WithCancel(context.Background())` — neither derived from the other. Correct.
- `Start` uses `b.loopCtx` for `sem.Acquire` and loop exit (bot.go:115). Handlers receive `b.handlerCtx` (line 121).
- `Stop()` sequence (bot.go:130-159):
  1. close `stopCh` (idempotent via select)
  2. `b.loopCancel()` — stops new dispatches
  3. `b.sem.Acquire(drainCtx, maxConcurrent)` with 10s timeout — waits for in-flights
  4. `b.handlerCancel()` — fires AFTER drain
  5. `flushOffsetToRedis()` final flush
- main.go shutdown (lines 116-128): `bot.Stop()` called BEFORE `app.ShutdownWithContext` AND BEFORE `dbPool.Close()` / `rdb.Close()`. In-flight handlers complete while DB pool still open. Correct ordering.
- The nested-timeout concern (main.go has its own 10s, bot has 10s drain) is benign: `bot.Stop()` is a plain blocking call, not deadlined by main. Worst case: bot's 10s drain runs to completion, THEN the HTTP shutdown gets its fresh 10s via `shutdownCtx`.

### H1 verification — loadUser fail-closed
- middleware.go:123-131: DB error branch sends friendly `⚠️ Hệ thống tạm thời gặp sự cố` reply and returns nil. No `next()` call. Ban check cannot be bypassed.
- Dev-mode synthetic user (line 92-100): `BotUser{Language: "vi", IsBanned: false}` — `ID` is zero UUID (ok for middleware flow which doesn't dereference it in Phase 01). `IsBanned=false` means banCheck passes through, which is correct for dev.
- Zero `tgID` path (line 103-105) still calls `next()` without a user — this is for updates with no `From.ID` (e.g. channel posts). banCheck's `UserFromCtx(ctx)` returns `(zero, false)`, so `ok && user.IsBanned` evaluates false → passes through. Safe for Phase 01 skeleton; becomes a ban-bypass vector again in Phase 02 when handlers do credits work. Informational, not blocking.

### H2 verification — CallbackQuery.Message nil-check
- update_helpers.go:64-65: `if u.CallbackQuery != nil && u.CallbackQuery.Message != nil`. Correct guard.
- router.go:83-85: `if update.CallbackQuery == nil || update.CallbackQuery.Message == nil { return nil }`. Early return before any field deref.

### H3 verification — detached ctx flush
- bot_concurrency_helpers.go:77-90: `flushOffsetToRedis` uses `context.WithTimeout(context.Background(), 2*time.Second)`. Not derived from loopCtx/handlerCtx.
- Called from ticker goroutine (line 67) AND from Stop() tail (bot.go:156).

### M1/M2/L1 verification
- M1: `Grep "banCheck(deps"` → 0 matches. `banCheck` (middleware.go:141) takes only `next` now.
- M2: BotUser struct (update_helpers.go:23-27) has `ID`, `Language`, `IsBanned` only. No `TelegramID` field. Comment explicitly documents YAGNI reasoning (line 22).
- L1: `Grep "singleflight"` → 0 matches anywhere in `services/api/`. Old comment gone.

## New findings
None critical or high. Observations below are informational only; do not block commit.

### N1 (low/info) — dev-mode synthetic user has zero UUID
`middleware.go:94-98`: synthetic `BotUser{}` has `ID == uuid.Nil`. Phase 02 handlers that reference `user.ID` in SQL would write zero-UUID rows. Acceptable for Phase 01 because no handler uses `user.ID` yet; call it out for Phase 02 planning — loadUser dev-mode should either skip `next()` or insert a synthetic UUID. Non-blocking.

### N2 (low/info) — `loopCtx`/`handlerCtx` cancels not deferred
`bot.go:130-159`: Stop() calls `loopCancel()` and `handlerCancel()` but doesn't wrap them in `defer` for panic safety. If `sem.Acquire` panics mid-drain, `handlerCancel` never fires and the goroutine leak risk is theoretical. Probability: near-zero (Acquire doesn't panic in practice). Informational.

### N3 (low/info) — `runOffsetFlusher` tick interval 1s
`bot_concurrency_helpers.go:19`: `offsetFlushInterval = time.Second`. Under heavy load (e.g., 50 updates/s), this writes Redis once per second even on steady-state — acceptable. Under idle traffic, `cur > lastFlushed` guard avoids redundant writes. Fine.

### N4 (low/info) — `state_test.go` and new tests do not exercise loop wiring
New tests cover `perUserLock` + `persistOffset` in isolation. The full `handleUpdate → middleware chain → persistOffset` flow has no integration test. Phase 02 plan should add one when middleware gains real behavior. Non-blocking for Phase 01.

## Approved items

- C1 fix is textbook — `sync.Map[int64]*userLock` with lazy init, time-based GC, clean test coverage.
- `persistOffset` CAS loop is correctly monotonic; out-of-order goroutines cannot regress the offset.
- Stop() sequence is clearly commented step-by-step (bot.go:138-156).
- `flushOffsetToRedis` nil-guards `deps.Rdb` AND zero offset — won't corrupt Redis with empty write.
- Concurrency test names self-document intent (`SerializesConcurrentUpdates`, `DifferentUsersRunConcurrently`).
- main.go shutdown ordering comment (line 116-117) accurately explains why bot.Stop precedes DB close.
- File-size discipline maintained: bot.go 195 lines, bot_concurrency_helpers.go 113 lines — both under 200.
- `go build ./...` + `go vet ./...` + `go test ./internal/bot/...` all clean. 11/11 tests pass (5 concurrency + 6 FSM).
- Race detector unavailable on Windows without cgo — not a blocker; CI runs Linux with `-race`.
- Dev-mode synthetic user short-circuit is pragmatic (dev without DB stays functional).

## Recommended next action

**commit-ready**

Suggested commit message:
```
feat(bot): phase 01 bot skeleton — long-poll loop, middleware chain, FSM store

- Per-user mutex serializes concurrent updates (not dedup) via sync.Map + 1h idle GC
- Poll offset persisted AFTER handler with CAS-max; ticker flush to Redis every 1s
- Decoupled loopCtx (stops dispatch) and handlerCtx (cancels only after drain)
- loadUser fail-closed on DB error; dev-mode synthetic user for local work
- CallbackQuery.Message nil-guards on updateChatID + handleCallback
- Detached context for Redis flush so shutdown doesn't abort write
- 11 tests pass (5 concurrency + 6 FSM); build + vet clean
```

## Unresolved questions

1. Phase 02 should revisit dev-mode synthetic user's zero UUID (N1) before wiring credits handlers.
2. Is an integration test exercising the full `handleUpdate → middleware → persistOffset` pipeline worth adding in Phase 02, or deferred? Recommend adding when Phase 02 introduces real middleware behavior.
3. CI `-race` coverage confirmed on Linux; Windows dev lacks cgo. No action needed unless cross-platform race verification becomes a requirement.

**Status:** DONE
**Verdict:** APPROVED
**Summary:** prior findings closed 9/9; new findings: crit 0, high 0, med 0, low/info 4
**Report:** E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay\reports\review-260425-0055-phase-01-fix-loop1.md
**Commit sha:** [pending — not yet committed]
**Next action:** commit-ready
