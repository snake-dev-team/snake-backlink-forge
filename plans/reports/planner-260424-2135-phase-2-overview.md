---
title: "Planner Report — Phase 2 Telegram Bot + Wallet + SePay"
role: planner
date: 2026-04-24
phase: 2
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
status: DONE_WITH_CONCERNS
---

# Phase 2 Plan — Summary Report

## Phase count + todo totals
- **Phases:** 10
- **Total estimated effort:** 38h
- **Total todo items across phases:** ~115 (avg ~11/phase; test phase heaviest at 15)
- **New DB migration:** 1 (`20260424002_phase2.sql` — 2 unique partial indexes + 2 tables `support_tickets` + `referrals`)
- **New packages:** `bot/`, `bot/commands/`, `bot/keyboards/`, `bot/templates/`, `service/`, `integration/sepay/`, `testutil/`, `e2e/`
- **External deps added:** `go-telegram-bot-api/v5`, `testcontainers-go`, `miniredis/v2` (test), `testify`, `phonenumbers` (optional)
- **sqlc queries added:** 8 files (users, keys, wallets, ledger, transactions, audit, admin_stats, support+referrals)

## Critical design decisions (locked)

1. **Telegram lib = `go-telegram-bot-api/v5`** — respects MASTER_PROMPT §1.2 locked ADR; minimal wrapper matches our middleware arch.
2. **FSM = Redis JSON string per `telegram_id`, TTL per-state, `singleflight` for per-user serialization.** No Postgres state table — FSM is UI context, not business data.
3. **Credit-grant idempotency = CAS via `UPDATE transactions WHERE status='pending' RETURNING` inside pgx.Tx with READ COMMITTED + row lock.** The `transactions` row IS the idempotency token. **No change to existing stored procs required.** Double-credit physically impossible.
4. **Topup intent idempotency = Redis lock (UX) + `UNIQUE (user_id, package_code) WHERE status='pending'` partial index (correctness).** Belt + suspenders.
5. **SePay auth = `Authorization: Apikey <token>`** (NOT Bearer, confirmed via docs fetch). Compare via `crypto/subtle.ConstantTimeCompare`. Return `200 {success:false}` on mismatch to avoid SePay's 7x retry storm.
6. **Trial gate = phone-uniqueness + phone-required, NOT account-age** — Bot API does not expose Telegram account creation date. Replaced master prompt §5.5 stale code with: unique partial index `idx_users_phone_trial` on `users(phone_e164) WHERE trial_used=TRUE`.
7. **Packages = Go struct constant** in `service/packages.go` — single source for menu render AND webhook grant amount. Package prices SNAPSHOTTED into `transactions` row on create; webhook reads row, not live registry (prevents price-change midway credit discrepancy).
8. **Templates = Go `map[lang]map[key]string` + `text/template`** — compile-time key safety via constants in `keys.go`, VN default + EN fallback, sync.Map cached. Copywriter agent fills final VN "bắt tai" copy in `/ck:cook`.
9. **Admin role = env-driven int slice** (`ADMIN_TELEGRAM_IDS`), silent ignore for non-admins (no info leak). Every admin write → `audit_log`.
10. **Test harness = testcontainers-go Postgres + miniredis, fake `tgbotapi.BotAPI` interface**. Suite D (webhook atomic grant with 10x concurrent) is the CRITICAL gate.

## Open questions requiring user input

1. **SePay merchant IP allowlist** — should we whitelist SePay webhook source IPs at Fly firewall level? User action: obtain SePay's published webhook egress IPs. DEFERRED to Phase 10 deploy.
2. **Bank account + code** — need values for `SEPAY_BANK_ACC` + `SEPAY_BANK_CODE` env before phase 05 cook. User action: register SePay merchant account.
3. **Admin Telegram IDs** — need at least 1 ID for `ADMIN_TELEGRAM_IDS` env. User action: get via @userinfobot.
4. **Installer URL** — phase 07 `/download` returns env `INSTALLER_URL`; actual binary lands in Phase 7. Placeholder OK until then. Confirm copy wording with Phase 7 team.
5. **Referral payout rules** — `/ref` MVP this phase shows code + total_referred only. Payout logic (credits on referrer's successful top-up?) DEFERRED to Phase 3+. User: confirm this scope cut.
6. **EN copy quality bar** — copywriter agent in `/ck:cook` handles VN "bắt tai" tone. EN placeholders in phase 09 are professional-neutral; should EN ALSO get native copywriter pass or is our scaffold sufficient? Default assumption: scaffold sufficient for v1, iterate post-launch.
7. **Stored proc signature stability** — existing `grant_credits` / `consume_credits` signatures locked. Any future schema change (e.g., new `metadata jsonb` param) requires migration + coordinated service refactor. Flagged for tech-debt backlog.

## Red flags for red-team review

1. **RACE WINDOW — existing stored proc has no idempotency.** Mitigation via caller-side CAS on `transactions` status is correct AS LONG AS every caller uses it. If a future endpoint calls `grant_credits` directly without the CAS wrapper, double-credit bug re-enabled. **Action for code-reviewer: add lint check / grep rule: any call to `grant_credits` MUST be inside a pgx.Tx alongside a status-check UPDATE.** Document this pattern in `docs/code-standards.md`.

2. **SEPAY TOKEN ROTATION BLIND SPOT.** If SePay rotates API key and we don't update env immediately, ALL incoming webhooks silently return `200 success=false`. User sees "bank received" but wallet not credited → support flood. **Mitigation proposed: audit_log event=`sepay_auth_fail` with burst alert (10+ in 5min → Telegram DM to admin). Implementation in phase 08 admin stats; verify alert mechanism actually wired in phase 10 test G.**

3. **PHONE NUMBER AS IDENTITY — VN carrier porting + recycling.** Cell numbers in VN get recycled after 3-6 months unused. User A's trial-consumed phone recycles to User B → User B blocked from trial. Low-frequency but non-zero. **Accepted risk for Phase 2; long-term: add `phone_verified_at` timestamp + optional re-verify every 6mo. Deferred.**

4. **TEST SUITE D PRESSURE REQUIRED.** Concurrent webhook replay test (10 goroutines) might pass under low CPU contention but miss race under real load. **Recommendation: run Suite D with `-count=100 -cpu=1,2,4,8` in CI to stress-test scheduler.**

5. **FSM STATE CORRUPTION.** If Redis flushes (maintenance, OOM), all in-flight `topup_waiting` FSMs lost. Users see "nothing happened" after paying. **Mitigation: webhook flow does NOT depend on FSM — grants from DB pending row regardless of FSM state. FSM is UX-only. Notified async goroutine still reaches user via Telegram message even without FSM.** Safe-by-design; documented in phase 01 insights.

6. **TELEGRAM MESSAGE DELIVERY IS BEST-EFFORT.** After `COMMIT` of webhook, the notify goroutine may fail (Telegram API down, user blocked bot, network). User won't know top-up succeeded until they `/balance`. **Mitigation: audit_log captures success regardless; user can self-serve check via `/balance` or `/history`. Acceptable.**

7. **SHARED PROCESS = SHARED FATE.** Bot and API in same binary — if bot goroutine panics-crash-infinite-loops, API also degrades. Recovery middleware catches per-update panics but goroutine leaks possible. **Mitigation: semaphore cap (50) + context timeout (25s) + panic recovery. Monitor via memory profile in phase 10 load test.**

8. **MIGRATION 20260424002 ORDERING.** New migration runs AFTER 20260424001 (current). If Phase 1 schema not applied, Phase 2 migration fails. **Mitigation: migration smoke test Suite A runs entire sequence up then down. Covered.**

9. **PACKAGE PRICE CHANGE MID-PENDING.** If admin changes `Packages` map (code redeploy) while user has pending tx → user paid OLD amount, webhook grants OLD amount (snapshotted in tx row). Correct behavior BUT inconsistent with new pricing. **Accepted: snapshot is the right answer; document in SECURITY.md.**

10. **REFERRAL PROCESSING RACE.** `/start ref_XXXXXX` called TWICE by same user → `UPDATE users SET referred_by = ... WHERE referred_by IS NULL` is idempotent. But `IncrementReferralCount` called twice inflates count. **Mitigation: wrap in same txn as `referred_by` UPDATE, guard with `WHERE referred_by IS NULL` on UPDATE → only runs once.** Called out in phase 07.

## Checklist — behavioral rules

- [x] Explicit data flows documented per phase (§ Architecture in each)
- [x] Dependency graph complete (plan.md dependency diagram)
- [x] Risk assessed per phase (table in each phase doc)
- [x] Backwards compatibility: Phase 1 schema untouched; new migration additive only
- [x] Test matrix defined (phase 10 Suite A-G)
- [x] Rollback plan: `make migrate-down` reverts 20260424002 cleanly (tested in Suite A)
- [x] File ownership — no two phases touch same file without explicit modification notes
- [x] Success criteria measurable per phase

## Plan dir tree

```
plans/260424-2135-phase-2-telegram-bot-wallet-sepay/
├── plan.md                                            (overview, 80 lines, dep graph)
├── research/
│   └── research-01-library-and-concurrency-decisions.md
├── phase-01-bot-skeleton.md
├── phase-02-user-service.md
├── phase-03-key-service.md
├── phase-04-wallet-ledger.md
├── phase-05-transactions-topup.md
├── phase-06-sepay-webhook.md                          (CRITICAL — idempotent CAS design)
├── phase-07-history-support.md
├── phase-08-admin-commands.md
├── phase-09-message-templates.md
├── phase-10-integration-tests.md                      (CRITICAL — 10x concurrent webhook test)
```

## Recommendation before `/ck:cook`

1. **Red-team review critical sections:** phase-06 (SePay webhook atomic grant + auth) + phase-10 Suite D. Get fresh eyes on the CAS pattern.
2. **Verify user has provisioned:** Telegram bot token, SePay merchant credentials + webhook token, admin TG IDs. Plan assumes env populated.
3. **Local runtime smoke:** `make migrate-up` on current branch (already done — Phase 1 verified). No action needed.
4. **Decide on library pin:** `github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1` is 2+ years old but stable. Acceptable. If user prefers more-active `mymmrac/telego`, revisit ADR §1.2.

**Status:** DONE_WITH_CONCERNS — plan ships. Concerns are all pre-existing product/ops risks, not plan defects. Red-team review of phase-06 and phase-10 Suite D strongly recommended before cook.
