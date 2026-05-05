---
title: "Phase 2 Research — Library + Concurrency Decisions"
type: research-memo
created: 2026-04-24
---

# Research: Library + Concurrency Decisions for Phase 2

## 1. Telegram library pick

**Options evaluated**
| Lib | Status | Pros | Cons |
|---|---|---|---|
| `go-telegram-bot-api/v5` (tgbotapi) | v5.5.1 (stable, wrapper) | Stable, minimal, idiomatic Go; net/http | No built-in handler router; manual FSM; less active maintenance |
| `mymmrac/telego` | pre-v1, very active 2026 | fasthttp + go-json; predicate handlers; opinionated | fasthttp pulls heavy dep (already in project via Fiber — net plus); not yet v1 |
| `go-telegram/bot` | v1.x | net/http; handler pattern | Smaller community |

**Decision: `go-telegram-bot-api/v5` (tgbotapi)**

**Rationale (KISS):**
- MASTER_PROMPT §1.2 LOCKED the decision: `go-telegram-bot-api/v5 (long poll)`. Per ADR rule "Locked, KHÔNG thay đổi trong quá trình implement".
- Minimal wrapper matches our existing middleware-centric Fiber architecture — we own the router/FSM anyway (Redis-backed).
- fasthttp is already transitively imported by Fiber — but relying on Fiber's fasthttp for webhook + separate net/http for bot long-poll is cleaner isolation.

**Acknowledged tradeoff:** tgbotapi receives fewer updates. If we hit a missing Bot API method (Bot API 7.x features), we wrap the raw HTTP call ourselves. Not blocking for Phase 2 scope.

## 2. Redis FSM state storage

**Decision: Single Redis string per user, JSON payload, TTL controlled by state.**

Key: `tg:state:<telegram_id>`
Value:
```json
{
  "state": "buy_confirming",
  "data": {"package_code": "premium_pro_200"},
  "expires_at": 1745510000
}
```

TTL:
- Default 30 min via `SETEX`
- `topup_waiting`: 24h (longer — user may close app, come back, pay later)
- `idle`: no key (absence = idle; saves Redis memory for 99% of users who chat rarely)

**Concurrency safety:** per-user commands are serialized via `singleflight.Group` keyed on `telegram_id` — prevents two goroutines handling two updates simultaneously from a fast-double-tap. golang.org/x/sync/singleflight is already transitively pulled in.

**Why not SERIALIZABLE Postgres txn instead of Redis FSM?**
- FSM state is ephemeral UI context, not business data. Redis is purpose-fit. Ledger integrity uses Postgres txn separately.

## 3. pgx transaction isolation for credit-grant critical section

**Options:**
- `READ COMMITTED` (default) + `SELECT ... FOR UPDATE` on `transactions` row
- `REPEATABLE READ`
- `SERIALIZABLE` — retry on 40001

**Decision: READ COMMITTED + `SELECT ... FOR UPDATE` on `transactions` row, using `UPDATE ... WHERE status='pending' RETURNING` as an atomic CAS.**

**Rationale:**
- `SERIALIZABLE` pays a throughput tax and requires retry loops on 40001 — overkill for our webhook qps (max tens/minute).
- `READ COMMITTED` + row lock gives us: (a) exclusive access to the txn row, (b) atomic transition check via `RETURNING`, (c) no retry loop.
- The stored procs `grant_credits` / `consume_credits` take their own row locks on `wallets` — no cross-row deadlock because update order is always `transactions → wallets` (same direction).

**Go idiom:**
```go
tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
defer tx.Rollback(ctx) // safe no-op if committed

cmdTag, err := tx.Exec(ctx, `
  UPDATE transactions
  SET status='paid', paid_at=NOW(), metadata = metadata || $2::jsonb, updated_at=NOW()
  WHERE provider_ref=$1 AND status='pending'
`, providerRef, rawPayload)
if cmdTag.RowsAffected() == 0 {
    return ErrAlreadyProcessed // either not found or already paid — webhook retry
}

// now it's ours; grant credits. Both pools in same txn.
if premium > 0 {
    _, err = tx.Exec(ctx, `SELECT grant_credits($1, 'premium', $2, 'topup', 'transaction', $3)`,
        userID, premium, txID)
    if err != nil { return err }
}
if standard > 0 {
    _, err = tx.Exec(ctx, `SELECT grant_credits($1, 'standard', $2, 'topup', 'transaction', $3)`,
        userID, standard, txID)
    if err != nil { return err }
}

if err := tx.Commit(ctx); err != nil { return err }
// notify Telegram AFTER commit (fire-and-forget goroutine)
```

**Why we DO NOT need to modify the existing `grant_credits` stored proc:**
- The idempotency gate is the `UPDATE transactions ... WHERE status='pending'` CAS. Second concurrent webhook sees `RowsAffected=0`, returns early, never calls `grant_credits`. Double-credit impossible.
- Cleaner than adding idempotency keys inside the stored proc because (a) the `transactions` row IS the idempotency token, (b) we avoid stored-proc schema changes.

## 4. `/topup` idempotency strategy

**Scenario:** user spams the same package button within seconds.

**Decision: Redis lock + unique partial index, both.**

Layer A — optimistic Redis lock (UX-friendly fast-path):
- Key: `topup_lock:<user_id>:<package_code>`, TTL 30s, `SET NX`
- First caller creates transaction row + QR. Subsequent callers within 30s get cached QR from Redis key `topup_qr:<user_id>:<package_code>` (TTL 24h to match `topup_waiting`).

Layer B — DB guard (correctness):
- Add to migration `20260424002_phase2.sql`:
  ```sql
  CREATE UNIQUE INDEX idx_tx_user_pkg_pending
    ON transactions(user_id, package_code)
    WHERE status = 'pending';
  ```
- Second pending txn for same (user, package) fails with `23505 unique_violation` → caller treats as "already have pending, show existing QR".

**Why both:** Redis is fast but can flap on network partition. Postgres unique constraint is the ground truth. Redis gets 99% of traffic; DB catches the 1% race.

## 5. SePay Authorization scheme — final reconciliation

Per SePay docs fetched 2026-04-24:
- Header: `Authorization: Apikey <token>` (NOT Bearer)
- SePay retries up to 7x with Fibonacci backoff on non-success
- Response should be 200 or 201 with body `{"success": true}`

**Decision: Return `200 {"success": false, "reason": "invalid_signature"}` on auth fail.**

**Rationale:**
- Returning 401 triggers SePay retry storm (up to 7x). We don't want an attacker with stolen Telegram screenshot to hammer our endpoint by spoofing.
- Returning 200 + `success: false` tells SePay "ack received, don't retry" while our internal logs show the auth failure.
- Real legit webhooks pass auth → 200 + `success: true` + credit granted.
- Unknown transaction ref (e.g., manual user transfer without order code) → 200 + `success: true` + log to `audit_log` event=`sepay_unmatched_transfer` for manual reconciliation.

**Timing-safe compare:** `crypto/subtle.ConstantTimeCompare([]byte(header), []byte(envToken))` only after prefix `Apikey ` stripped.

## 6. Trial account-age heuristic

Telegram Bot API does NOT expose user account creation date. Master prompt §5.5 has stale code.

**Decision: 3-gate defense instead of account age:**
1. `phone_e164` contact required (Telegram `request_contact`).
2. Normalize phone → `phone_e164` canonical form; reject if already used by another user with `trial_used=TRUE` (unique partial index).
3. Per-IP rate limit on `/start` new-user path: 5/hour (Redis sliding-window). Note: Telegram long-poll doesn't expose client IP — drop rule 3 and rely on 1+2.

**Extra signal (proxy for "new Telegram account"):** Telegram user IDs are roughly monotonic. Store `telegram_id` of first N users as a seed; if new `telegram_id` is wildly higher than the current rolling average jumped by 100M+ from ref, flag for review. DEFERRED — speculative, KISS says skip.

**Final trial gate (Phase 2):**
```go
// 1. phone provided
// 2. no other user with same phone_e164 has trial_used=TRUE
// 3. this user's trial_used=FALSE
// 4. user is not banned
```

## 7. QR generation

**SePay QR endpoint:** `https://qr.sepay.vn/img?acc=<acc>&bank=<bank_code>&amount=<N>&des=<desc>&template=compact`

**Decision: server-side URL construction, push as Telegram photo via `sendPhoto` with URL (Telegram fetches & caches).**

No CDN caching on our side (URL is ephemeral per-order). Bank + account from env `SEPAY_BANK_ACC` + `SEPAY_BANK_CODE`.

Order code format: `first_8_chars(provider_ref_uuid)`, uppercase. Stored in `transactions.provider_ref`.

SePay "content" field regex for extract: `/SBF\s+TOPUP\s+([A-F0-9]{8})/i`.

## Open Questions (non-blocking)

1. Does SePay require us to whitelist our server IP in their dashboard? → user action needed before test.
2. For admin commands, should audit log IP come from Telegram update source? Telegram bot API doesn't expose client IP on long-poll — IP field will be NULL for admin actions (acceptable, webhooks still log IP).
3. Ref program (`/ref`) — Phase 2 scope says include command; specifies referral code gen but not payout rules. Plan Phase 2 as MVP: show code + stats only, no payout logic (Phase 3+).
