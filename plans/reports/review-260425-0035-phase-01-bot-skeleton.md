# Code Review — Phase 01 Bot Skeleton

## Verdict
FIX_REQUIRED

## Summary
Skeleton is well-organized and builds/tests clean, but three correctness bugs block Phase 02: singleflight silently DROPS duplicate-user updates (not serializes them), offset is persisted BEFORE handler runs (data loss on crash), and handler ctx is a child of botCtx so shutdown cancels all in-flights instantly — making the 10s drain cosmetic. loadUser silent-failure path also bypasses the ban check.

## Critical findings

### C1 — singleflight drops duplicate-user updates instead of serializing
`services/api/internal/bot/bot.go:140`

```go
b.sf.Do(key, func() (interface{}, error) { //nolint:errcheck
    ...
    handler := buildChain(b.deps, route(b))
    if err := handler(ctx, b.api, update); err != nil { ... }
    return nil, nil
})
```

`singleflight.Do` does NOT serialize — it **deduplicates**. Per `pkg.go.dev/golang.org/x/sync/singleflight`: "If a duplicate comes in, the duplicate caller waits for the original to complete and **receives the same results**." The duplicate's `fn` never executes.

Impact: two updates from the same user arriving within the same window → only the first handler runs, the second is silently dropped. The closure captures `update` from the first invocation; the second update is never processed at all.

Concrete scenarios blocking Phase 02+:
- User types `/topup 100000` then `/topup 200000` quickly → second msg dropped
- User types `/start` twice → ok in this phase (dedup is beneficial), but in Phase 02 each `/start` may carry different contact-share timing
- Fat-finger `/balance` double-tap → one reply sent, second tap appears ignored from UX

Fix: replace `singleflight.Group` with per-user mutex (use `sync.Map[int64]*sync.Mutex` + LRU eviction) or a per-user chan dispatcher. The plan's intent ("per-user updates serialize") requires queue-semantics, not dedup-semantics.

Flag as CRITICAL because phase spec says "prevents double-tap race in `/start` trial grant" — current code happens to achieve that by silent-drop, but the same mechanism will break every other flow Phase 02+ adds.

### C2 — poll offset saved BEFORE handler runs; crash loses the update
`services/api/internal/bot/bot.go:94`

```go
case update, ok := <-updates:
    ...
    b.saveOffset(ctx, update.UpdateID+1)   // saved BEFORE handler dispatch
    ...
    go func() {
        defer b.sem.Release(1)
        b.handleUpdate(ctx, u)
    }()
```

If process OOM/crashes after saveOffset but before handler completes, the update is permanently lost on restart (Redis offset already advanced past it). Telegram `getUpdates` will never return it again.

While skeleton handlers are currently mostly stubs, Phase 02+ (loadUser upsert, trial grant) performs side effects that WILL break user trust if half-applied and then lost.

Fix: save offset AFTER handler returns. Either:
- (a) Move `b.saveOffset` into the goroutine AFTER `handleUpdate`, accepting slight reordering (batch offsets may go backward briefly — need `Max()` semantic in saveOffset)
- (b) Accumulate completed update_ids in a channel, have a single goroutine persist the running max
- Short-term: keep current behavior and mark handlers as idempotent in docs; file tracking ticket. But that's a load-bearing constraint Phase 02 must honor.

### C3 — handler parentCtx is botCtx; shutdown cancels all in-flights immediately, 10s drain is cosmetic
`services/api/internal/bot/bot.go:105,136,141` + `services/api/cmd/api/main.go:125-128`

```go
// main.go
botCancel()                // cancels botCtx
if bot != nil { bot.Stop() } // Stop() then waits 10s for sem drain
```

```go
// bot.go Start loop
b.handleUpdate(ctx, u)                       // ctx == botCtx
// handleUpdate
ctx, cancel := context.WithTimeout(parentCtx, handlerTimeout)  // child of cancelled botCtx
```

When `botCancel()` fires, every in-flight handler's `ctx.Done()` closes immediately because its parent is botCtx. Handlers mid-DB-write or mid-`api.Send` see context.Canceled and abort. `Stop()` then patiently waits 10s for semaphore drain — but handlers have already bailed out uncleanly.

Effect: user mid-transaction sees Telegram message not sent (api.Send respects ctx) and DB row possibly half-written. The whole point of a 10s drain is to let these complete. Currently the drain just waits for goroutines to notice ctx is cancelled and return.

Fix: decouple handler ctx from botCtx. Start loop should use botCtx for `sem.Acquire` and loop exit, but pass `context.Background()` (or a separate "handler parent ctx" cancelled only by Stop AFTER drain-timeout) into handleUpdate. Sequence:

```go
// Pseudocode
botCancel()         // only stops NEW updates from being dispatched
bot.Stop()          // waits up to 10s for in-flights, THEN cancels handlerCtx
dbPool.Close()      // safe — no in-flight handlers
```

Requires `Bot` to hold two contexts: `loopCtx` (cancelled on botCancel) and `handlerCtx` (cancelled only on hard-timeout).

## High findings

### H1 — loadUser silent-failure bypasses banCheck
`services/api/internal/bot/middleware.go:111-116`

```go
err := deps.Pool.QueryRow(ctx, `INSERT ... RETURNING ...`).Scan(...)
if err != nil {
    deps.Log.Warn("loadUser: upsert failed", ...)
    return next(ctx, bot, update)   // <-- proceeds WITHOUT user in ctx
}
```

If DB query fails (connection blip, timeout, deadlock), loadUser silently passes through. `banCheck` at line 128 then calls `UserFromCtx(ctx)` which returns `(zero, false)` → ban check is SKIPPED → banned user reaches the handler.

Impact: transient DB issues create a ban-bypass window. For skeleton this may feel theoretical, but Phase 02+ has real trial grants and Phase 03+ has credits spend. A banned user hitting the handler during a DB flake could fire a refund abuse path.

Fix: on upsert error, return a sentinel (don't call next). Optionally reply "temporary issue, try again" and return nil. Ban semantics should FAIL CLOSED — unknown user → treat as if banned.

### H2 — updateChatID may nil-deref on stale callback queries
`services/api/internal/bot/update_helpers.go:63-64`

```go
if u.CallbackQuery != nil {
    return u.CallbackQuery.Message.Chat.ID   // Message may be nil
}
```

Per tgbotapi, `CallbackQuery.Message` can be nil for callbacks on inaccessible/old messages. A panic here would be caught by `recoverMiddleware` but only if it's called from handler path; `replyText` → `updateChatID` in the recover path itself would panic again and the recover closure would only catch the new panic (but we're already past `defer recover`).

Actually more concerning: `replyText` is called from `recoverMiddleware` itself (line 48). If the original panic is in the chain and we try to reply via `updateChatID` which panics on nil `Message`, the outer recover handles it — retErr set and logged — but no reply sent to user and a second stack logged.

Fix: nil-check `u.CallbackQuery.Message`:
```go
if u.CallbackQuery != nil && u.CallbackQuery.Message != nil {
    return u.CallbackQuery.Message.Chat.ID
}
```

Same concern in `router.go:87` (`handleCallback` builds reply with `update.CallbackQuery.Message.Chat.ID`).

### H3 — saveOffset uses cancelled ctx during shutdown mid-batch
`services/api/internal/bot/bot.go:170-176`

When SIGTERM arrives mid-Start-loop, subsequent saveOffset calls see `ctx.Err() != nil`. `rdb.Set` returns context.Canceled silently, offset falls behind by N updates from the in-flight batch. On next boot those N are reprocessed.

Risk: low probability × low impact (reprocess acceptable if handlers idempotent — but per C2, we know they're not guaranteed to be).

Fix: use `context.Background()` with 2s timeout for saveOffset, OR move offset persistence out of the hot path entirely (see C2 fix).

## Medium findings

### M1 — `deps` param unused in `banCheck`
`services/api/internal/bot/middleware.go:125-136`

```go
func banCheck(deps *Deps) func(HandlerFunc) HandlerFunc {
    return func(next HandlerFunc) HandlerFunc {
        // deps never referenced
```

Dead parameter. If the intent is to later query DB for ban status (vs cached in user struct), add a comment. Otherwise drop the param.

### M2 — `BotUser.TelegramID` scanned but never consumed
`services/api/internal/bot/update_helpers.go:22-27` + `middleware.go:111`

```go
type BotUser struct {
    ID         uuid.UUID
    TelegramID int64   // scanned into but no reader
    Language   string
    IsBanned   bool
}
```

Plan note: "fields should be only what middleware actually uses NOW". `TelegramID` can be re-extracted via `updateTelegramID(update)` anywhere it's needed. Drop the field to honor YAGNI.

### M3 — `handleCallback` error swallow is confusingly documented
`services/api/internal/bot/router.go:82-86`

```go
cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
if _, err := api.Request(cb); err != nil {
    // Answering the callback query is best-effort; don't surface to caller.
    _ = err
}
```

The `_ = err` is a no-op — `err` is already scoped to the if-block and discarded on scope exit. Delete the body; comment alone suffices. Or better, log the error at debug level — silently swallowing makes Phase 02+ callback debugging painful.

### M4 — singleflight.Do return values dropped via `//nolint:errcheck`
`services/api/internal/bot/bot.go:140`

The lint suppressor hides that the 3-return form of `sf.Do` includes a `shared` bool that would actually surface C1 in logs (`shared=true` means "someone else's update got my result"). Capture it:

```go
_, _, shared := b.sf.Do(key, func() (interface{}, error) { ... })
if shared {
    b.deps.Log.Warn("bot: update coalesced by singleflight", zap.Int64("tg_id", tgID))
}
```

This doesn't fix C1 but makes its impact observable until then.

### M5 — `recoverMiddleware` `safeText` redaction leaks 0–200 char prefix
`services/api/internal/bot/middleware.go:36-47`

```go
if len(t) > 200 { t = t[:200] + "[redacted]" }
```

For a contact share (`update.Message.Contact`), `Message.Text` is empty and won't leak — fine. But `Message.Contact.PhoneNumber` (not `Text`) holds the phone. Redaction only applies to `.Text`. Acceptable for skeleton but worth a comment because Phase 02 adds contact-share handling which writes `Contact.PhoneNumber` — any panic there could be logged with phone via zap.Any("recover", r) if the panic message embedded it. Low probability.

### M6 — `state.Load` doc-string wraps goredis.Nil but test checks errors.Is
`services/api/internal/bot/state.go:42-48` + test:66-69

Works correctly (fmt.Errorf with %w preserves errors.Is). But the message "no state for %d" is redundant with the sentinel — caller either cares about `errors.Is(err, goredis.Nil)` (yes) or doesn't (treats as DB error). The %d format just noises up logs. Consider returning `goredis.Nil` directly to simplify.

## Low / nits

### L1 — `handleUpdate` comment claims singleflight "prevents double-tap races"
`services/api/internal/bot/bot.go:135`. See C1 — actually causes silent drops. Fix comment or code.

### L2 — `Bot.sf` field is zero-valued (not initialized in `New`)
`services/api/internal/bot/bot.go:38,56-62`. Fine — `singleflight.Group` zero-value is usable. Informational.

### L3 — `pollOffsetKey` is shared constant
`services/api/internal/bot/bot.go:30`. Good.

### L4 — `replyText` swallows `bot.Send` error without logging
`services/api/internal/bot/update_helpers.go:54`. Acceptable for recover-path (don't spam stderr), but in happy path it masks rate-limit errors from Telegram. Consider accepting a logger param or returning error.

### L5 — No test for `loadOffset` / `saveOffset` round-trip
No test coverage for offset persistence. Trivial to add with miniredis. Would have caught C2 by design-check.

### L6 — `cmdHelp` and `cmdStart` strings are hard-coded Vietnamese
`services/api/internal/bot/router.go:12,65`. Phase 09 handles i18n templates. Acceptable placeholder but the i18nMiddleware extracts lang and the commands don't use it — dead code path until Phase 09. Add a `// TODO(Phase 09)` only if it aids comprehension; otherwise leave.

### L7 — `BotUser.ID` type `uuid.UUID` requires that pgx scans UUID column correctly
`services/api/internal/bot/update_helpers.go:23`. pgx/v5 scans UUID into `uuid.UUID` via the google/uuid driver — works if the right pgx UUID codec is registered. Verify during Phase 02 smoke test (scan may fail silently and send err to loadUser's log-only branch, then bypass ban per H1).

## Plan spec alignment

| Requirement | Status | Evidence |
|---|---|---|
| Long-poll offset in Redis `tg:poll_offset` | OK (timing bug: C2) | bot.go:30,94 |
| Route commands, callbacks, contact shares | OK | router.go:25-40 |
| FSM Load/Save/Clear signatures match spec | OK | state.go:41,60,81 |
| Graceful shutdown 10s drain | PARTIAL (C3: drain cosmetic) | bot.go:28,122 |
| Panic recovery per update | OK | middleware.go:28-55 |
| Semaphore(50) cap | OK | bot.go:24,59 |
| Singleflight per telegram_id | WRONG SEMANTICS (C1) | bot.go:140 |
| context.WithTimeout(25s) per handler | OK (but parent wrong: C3) | bot.go:26,141 |
| Structured logs (tg_id, command, latency, result) | OK | middleware.go:70-76 |
| /ping → "pong" | OK | router.go:57-61 |
| `TELEGRAM_BOT_TOKEN` empty → API boots | OK | bot.go:45-47, main.go:70-89 |
| FSM test passes `-race` | Skipped locally (no GCC); CI runs `-race` | ci.yml:49 |

## Tests adequacy

- 6 FSM tests cover happy path + edge cases (missing, cleared, TTL, zero-TTL). Good.
- Missing: offset load/save round-trip test (see L5).
- Missing: concurrency test for Save — multiple goroutines Save same key at same TTL → last-write-wins is expected; no assertion. Not blocking.
- Missing: tests for the `bot.Start` lifecycle (could use mock BotAPI via interface extraction, but heavy — skip for skeleton).
- Missing: middleware unit tests (recover catches panic, banCheck short-circuits). Given C1/H1 above, these would have caught the bugs; strongly recommend adding Phase 02.
- `-race` runs only in CI on Linux. Skeleton passes without race warnings in current smoke — but Linux CI is the source of truth.

## Approved items (what's done right)

- File-size discipline: each file ≤ 200 lines; `update_helpers.go` split is well-justified.
- Context keys via unexported `ctxKey` type (prevents cross-package collision).
- `ErrBotDisabled` sentinel + `errors.Is` check in main.go (main.go:80).
- Redis nil-client defensive checks in `loadOffset`/`saveOffset` (bot.go:158,171).
- `loadUser` handles nil pool and zero tgID gracefully.
- `Stop()` idempotent via closed-channel check (bot.go:114-119).
- `Save` rejects zero TTL — prevents accidental permanent FSM keys.
- Bot token never logged; only `api.Self.UserName` surfaces (bot.go:54). Searched token references — clean.
- DB schema matches middleware INSERT columns exactly (telegram_id, telegram_username, language, is_banned, id).
- Comments explain WHY not WHAT in most places.
- Package docstrings in every file.
- `go build ./...` + `go vet ./...` + `go test ./internal/bot/...` all clean.
- CI has `-race` enabled for Linux (ci.yml:49).

## Recommended actions before commit

1. **C1** — Replace `singleflight.Group` with per-user mutex map or chan dispatcher. Current code silently drops same-user duplicates. Plan spec intent is serialization, not dedup. Do not merge as-is.
2. **C2** — Move `saveOffset` to AFTER `handleUpdate` returns. Persist offset only on successful handle. Handlers must declare idempotency explicitly (doc comment) since Telegram dedup by update_id is fallible.
3. **C3** — Decouple `handlerCtx` from `botCtx`. Start loop: use botCtx for loop control; pass a separate, shutdown-aware parent ctx to handleUpdate that lives through Stop()'s drain window.
4. **H1** — loadUser on DB error: fail closed. Don't call `next()` — reply "service unavailable" and return. Ban semantics must not be bypassable.
5. **H2** — Nil-check `CallbackQuery.Message` in `updateChatID` and in `handleCallback` reply path.
6. **H3** — Use detached `context.Background()` + short timeout in `saveOffset`.
7. **M1** — Drop unused `deps` param from `banCheck`, or document intent.
8. **M2** — Drop `BotUser.TelegramID` field (YAGNI).
9. **M4** — Capture `shared` from `sf.Do` and log when true (observability until C1 fixed).
10. **L1** — Update comment at bot.go:135 once C1 is fixed.
11. Add unit tests for middleware (recover, loadUser DB-fail, banCheck) to prevent regression.
12. Re-run `go build`, `go vet`, `go test ./... -race` (Linux CI will enforce).

## Unresolved questions

- Should the singleflight-replacement use an unbounded per-user mutex map (risk: map grows unbounded over months of traffic) or bounded LRU (adds complexity)? Suggest LRU size=10k with 1h TTL eviction, details in Phase 02 pre-work.
- Does Phase 02 plan already account for loadUser failure semantics (H1)? If so, fix can be deferred with explicit risk note. If not, must fix now.
- Is the 30s GetUpdates timeout (bot.go:73) intentional vs tgbotapi default? No impact, just verify.
