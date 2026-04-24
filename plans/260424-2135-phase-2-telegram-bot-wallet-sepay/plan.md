---
title: "Phase 2 — Telegram Bot + Wallet + SePay"
description: "Bot package, user/key/wallet services, SePay webhook with idempotent credit grant, admin commands, integration tests."
status: pending
priority: P1
effort: 38h
branch: dev
tags: [phase-2, telegram, wallet, sepay, go, fiber]
created: 2026-04-24
---

# Phase 2 Overview

Scope from MASTER_PROMPT §13 Phase 2. Build bot package inside `services/api` binary (shared process, shared DB/Redis). Unblocks Phase 3 (extension integration) by shipping key issuance + credit top-up.

## Phases

| # | File | Owner files | Effort | Status |
|---|---|---|---|---|
| 01 | [phase-01-bot-skeleton.md](./phase-01-bot-skeleton.md) | bot/bot.go, bot/router.go, bot/state.go, bot/middleware | 4h | pending |
| 02 | [phase-02-user-service.md](./phase-02-user-service.md) | service/user_service.go, bot/commands/start.go | 4h | pending |
| 03 | [phase-03-key-service.md](./phase-03-key-service.md) | service/key_service.go, bot/commands/key.go + regenkey.go, util/token.go | 3h | pending |
| 04 | [phase-04-wallet-ledger.md](./phase-04-wallet-ledger.md) | service/wallet_service.go, bot/commands/balance.go | 3h | pending |
| 05 | [phase-05-transactions-topup.md](./phase-05-transactions-topup.md) | service/transaction_service.go, bot/commands/buy.go + topup.go, integration/sepay/qr.go | 5h | pending |
| 06 | [phase-06-sepay-webhook.md](./phase-06-sepay-webhook.md) | api/handlers/webhook.go, integration/sepay/verify.go, integration/sepay/payload.go | 5h | pending |
| 07 | [phase-07-history-support.md](./phase-07-history-support.md) | bot/commands/history.go + support.go + download.go + ref.go + language.go | 4h | pending |
| 08 | [phase-08-admin-commands.md](./phase-08-admin-commands.md) | bot/commands/admin.go, service/audit_service.go | 3h | pending |
| 09 | [phase-09-message-templates.md](./phase-09-message-templates.md) | bot/templates/messages.go + renderer.go + keys.go | 2h | pending |
| 10 | [phase-10-integration-tests.md](./phase-10-integration-tests.md) | tests (testcontainers-go) | 5h | pending |

## Dependency Graph

```
01 Bot skeleton ──┬──▶ 02 User (/start) ──▶ 03 Key (/key /regenkey)
                  │                          │
                  │                          ▼
                  ├──▶ 04 Wallet (/balance) ─┼──▶ 05 Transactions (/buy /topup) ─▶ 06 SePay webhook
                  │                          │
                  │                          ├──▶ 07 History / Support / Download / Ref / Language
                  │                          │
                  └──▶ 08 Admin ─────────────┤
                                             │
                    09 Templates ◀───[referenced by all command files; write last]
                                             │
                                             ▼
                          10 Integration tests (needs 01-09 done)
```

## Critical design decisions (locked)

- **Telegram lib:** `go-telegram-bot-api/v5` (long poll). Bot + API share single process, single Postgres pool, single Redis.
- **FSM:** Redis string per `telegram_id`, JSON payload, per-state TTL. `singleflight` for per-user serialization.
- **Credit grant idempotency:** CAS via `UPDATE transactions WHERE status IN ('pending','cancelled') RETURNING` inside pgx.Tx, READ COMMITTED + row lock. Double-credit impossible without modifying existing stored proc. Cancelled orders matched by late webhook transition to `recovered_by_late_payment` (Q2).
- **Topup idempotency:** Redis lock (UX) + `UNIQUE (user_id, package_code) WHERE status='pending'` partial index (correctness).
- **SePay auth:** `Authorization: Apikey <token>` with `subtle.ConstantTimeCompare`. Return `200 {success:false}` on mismatch to avoid SePay retry storms.
- **SePay account binding (Q3):** single-bank Phase 2 — webhook must match both `accountNumber == env.SEPAY_BANK_ACCOUNT` and `gateway == env.SEPAY_BANK_CODE`. Multi-bank deferred to Phase 10+.
- **SePay webhook rate limit (Q4):** 20 req/sec/IP via Redis INCR fixed-window (`webhook_rl:<ip>` TTL 1s). Exceed → 429.
- **Webhook error classification (F3):** auth/business → 200 success=false (no retry); lock contention (40P01/40001/ctx deadline) → 200 queued to Redis retry list; infra/pool → 503; panic → 500 + alert.
- **Over-payment (Q1):** auto-credit excess as bonus using package per-credit rate (floor). Flag `manual_review=true` metadata only if excess ≥ 50,000đ; DM admin if ≥ 10,000đ. New ledger event `topup_excess`.
- **Admin DM alerting (Q5):** buffered channel `adminAlertCh` cap=100 + single consumer goroutine spawned in `main.go`. Non-blocking send; drop on full.
- **Trial gate (F4):** phone required + `phone_e164` unique across `trial_used=TRUE` users (not account-age — Bot API doesn't expose). Atomic via `INSERT ... ON CONFLICT ... WHERE users.trial_used=FALSE` + partial index as enforcement. No `COUNT(*)` pre-check.
- **Order code (F2):** 12-hex uppercase from first 6 bytes of UUID. Regex `(?i)` for case-insensitive extraction; `strings.ToUpper(match[1])` before DB query. Partial UNIQUE on `provider_ref` limited to active states (`pending`, `paid`, `recovered_by_late_payment`).
- **Messages:** `map[lang]map[key]string` + `text/template` interpolation, VN default with EN fallback.

## Command inventory (12 total, Phase 2)

`/start` `/key` `/balance` `/buy` `/topup` `/history` `/download` `/support` `/regenkey` `/ref` `/language` `/admin *`

**Deferred:** `/campaigns` → Phase 4+ (requires Campaign API CRUD endpoints).

## Migration requirements

Two new migrations (ordered after `20260424002_seed_dorks.sql`):

### `services/api/internal/migrations/20260424003_phase2_indexes.sql`
- `CREATE UNIQUE INDEX idx_tx_user_pkg_pending ON transactions(user_id, package_code) WHERE status='pending';`
- `CREATE UNIQUE INDEX idx_users_phone_trial ON users(phone_e164) WHERE trial_used=TRUE AND phone_e164 IS NOT NULL;`
- `CREATE TABLE IF NOT EXISTS support_tickets (...)` for `/support` command.
- `CREATE TABLE IF NOT EXISTS referrals (...)` minimal MVP: referrer_user_id, code, created_at.

### `services/api/internal/migrations/20260424004_phase2_schema_deltas.sql` (**new — from F2 + Q1 + Q2**)
- `ALTER TYPE ledger_event_type ADD VALUE 'topup_excess';` (Q1)
- `ALTER TYPE transaction_status ADD VALUE 'cancelled';` (Q2 — user cancel retains provider_ref for recovery)
- `ALTER TYPE transaction_status ADD VALUE 'recovered_by_late_payment';` (Q2)
- `ALTER TABLE transactions DROP CONSTRAINT transactions_provider_ref_key;` (drop unconditional UNIQUE)
- `CREATE UNIQUE INDEX idx_tx_provider_ref_active ON transactions(provider_ref) WHERE status IN ('pending','paid','cancelled','recovered_by_late_payment');` (partial UNIQUE, active states include `cancelled` so late payment matches)
- **Down irreversibility:** Postgres cannot remove enum values without recreating the type. Down migration documents this (DROP INDEX + ADD UNIQUE constraint back; enum values persist — acceptable since new values unused post-revert).

## Success criteria (Phase 2 done means)

- [ ] Bot answers all 12 listed commands on real Telegram with dev token
- [ ] `/start` grants 5 Standard credits atomically, ledger row exists, trial gate blocks 2nd attempt (partial-index enforcement)
- [ ] SePay webhook credits wallet exactly once under 10x concurrent replay
- [ ] Suite D stress: `-count=100 -cpu=1,2,4,8` matrix passes 100/100 iterations
- [ ] Over-payment path: bonus credits granted; `topup_excess` ledger event present
- [ ] Cancel-then-pay path: status flips to `recovered_by_late_payment`, full credits granted
- [ ] Wrong `accountNumber` or `gateway` → webhook 200 success=false + audit `sepay_account_mismatch` / `sepay_gateway_mismatch`
- [ ] Webhook rate limit 20/s/IP → 21st req in 1s window returns 429
- [ ] `go test ./... -race -cover` ≥ 80% in service/, bot/, integration/sepay/
- [ ] `docker compose up` + integration test green end-to-end
- [ ] 0 critical issues from `code-reviewer`

## Risk Assessment (plan-level)

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Multi-bank support needed post-Phase-2 | Med | Med | **Deferred to Phase 10+ (multi-bank support).** Current plan hard-binds single `SEPAY_BANK_ACCOUNT` + `SEPAY_BANK_CODE` |
| SePay source IPs not published in docs | Med | Low | **Pre-deploy blocker (phase-10):** research SePay docs at https://docs.sepay.vn. If static IPs absent, fall back to rate limit only |
| Down-migrate `20260424004` — enum values leak | Certain | Low | Postgres doesn't allow removing enum values; documented irreversibility. Values unused post-revert; acceptable |
| Provider_ref reuse by cancelled → new pending same code | Very Low | Low | Partial UNIQUE on active states only; collision across 48-bit space at 65K active ~negligible |
| Admin alert channel full (flood) | Low | Low | Non-blocking send with warn log; drops ordered by recency; no cascading failure |
