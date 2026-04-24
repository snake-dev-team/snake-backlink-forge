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
- **Credit grant idempotency:** CAS via `UPDATE transactions WHERE status='pending' RETURNING` inside pgx.Tx, READ COMMITTED + row lock. Double-credit impossible without modifying existing stored proc.
- **Topup idempotency:** Redis lock (UX) + `UNIQUE (user_id, package_code) WHERE status='pending'` partial index (correctness).
- **SePay auth:** `Authorization: Apikey <token>` with `subtle.ConstantTimeCompare`. Return `200 {success:false}` on mismatch to avoid SePay retry storms.
- **Trial gate:** phone required + `phone_e164` unique across `trial_used=TRUE` users (not account-age — Bot API doesn't expose).
- **Messages:** `map[lang]map[key]string` + `text/template` interpolation, VN default with EN fallback.

## Migration requirement

One new migration `services/api/internal/migrations/20260424002_phase2.sql`:
- `CREATE UNIQUE INDEX idx_tx_user_pkg_pending ON transactions(user_id, package_code) WHERE status='pending';`
- `CREATE UNIQUE INDEX idx_users_phone_trial ON users(phone_e164) WHERE trial_used=TRUE AND phone_e164 IS NOT NULL;`
- `CREATE TABLE IF NOT EXISTS support_tickets (...)` for `/support` command.
- `CREATE TABLE IF NOT EXISTS referrals (...)` minimal MVP: referrer_user_id, code, created_at.

## Success criteria (Phase 2 done means)

- [ ] Bot answers all listed commands on real Telegram with dev token
- [ ] `/start` grants 5 Standard credits atomically, ledger row exists, trial gate blocks 2nd attempt
- [ ] SePay webhook credits wallet exactly once under 10x concurrent replay
- [ ] `go test ./... -race -cover` ≥ 80% in service/, bot/, integration/sepay/
- [ ] `docker compose up` + integration test green end-to-end
- [ ] 0 critical issues from `code-reviewer`
