---
title: "Planner Round 2 — Phase 2 Patch Report"
role: planner
date: 2026-04-24
phase: 2
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
status: DONE
round: 2
---

# Phase 2 — Patch Round 2

Surgical patches applied per user directives Q1-Q6 + red-team F1-F6 (+ H5, H6, M3, M5, M6, L3). No scope expansion, no new subagents, no new top-level docs.

## Files touched (9)

| # | File | Sections patched | Delta one-liner |
|---|---|---|---|
| 1 | `plan.md` | Design decisions, Command inventory (new), Migration reqs, Success criteria, Risk Assessment (new) | Added Q1-Q6 locks + 12-cmd list; split migration into 003 (indexes) + 004 (schema deltas); added plan-level Risk table |
| 2 | `phase-02-user-service.md` | Key Insights, Data flow, Trial gate pseudocode, sqlc queries, Migration 003 section, Impl Steps, Todo, Risk | [F4] Replaced `checkTrialGate` with `FOR UPDATE` + conditional UPDATE, no `COUNT(*)` pre-check; dropped `CountOtherUsersWithPhoneTrialUsed` query; renamed migration to `20260424003` |
| 3 | `phase-03-key-service.md` | Requirements.Functional, Impl Steps #5, Todo, Risk | [H5] Added Redis rate limit 3/day `regen_rl:<user_id>` INCR+EXPIRE; short-circuits before FSM confirm |
| 4 | `phase-04-wallet-ledger.md` | Key Insights, Related Code Files, Migration section (new), Impl Steps, Todo | [F2+Q1+Q2] Added migration `20260424004_phase2_schema_deltas.sql` — ALTER TYPE (topup_excess, cancelled, recovered_by_late_payment) + DROP base UNIQUE + partial UNIQUE on active states; enum removal irreversibility documented |
| 5 | `phase-05-transactions-topup.md` | Key Insights, Non-functional, CreateTopupIntent pseudocode, CancelPendingTransaction sqlc, Impl Steps, Todo, Risk, Modify files | [F2] Order code: 12-hex uppercase via `hex.EncodeToString(uuid[:6])` + retry on `idx_tx_provider_ref_active` 23505 (3x); [Q2] cancel → status='cancelled' (not 'failed'); [Q3] rename env to `SEPAY_BANK_ACCOUNT` |
| 6 | `phase-06-sepay-webhook.md` | Key Insights, Requirements (Q3+Q4+F1+F2+Q2+H6), Handler flow pseudocode, Error classifier (new), WebhookService pseudocode, Payload regex ({12}), Admin alert channel wiring (new), Impl Steps, Todo, Success Criteria, Risk, Security, Related Code Files, Modify | [F1] 3-way amount branch with bonus grant + threshold flags; [F2] uppercase normalize; [F3] error classifier (200/429/503/500 matrix); [Q2] CAS widened to ('pending','cancelled') + CASE expression for status transition; [Q3] account+gateway match; [Q4] rate_limit middleware; [Q5] adminAlertCh non-blocking; [H6] notify ctx scoped to RootCtx+10s; [M4] full-length ip_hash |
| 7 | `phase-07-history-support.md` | /support section, Impl Steps, Todo, Risk, Security + /campaigns note | [Q6] Added deferral note for /campaigns; [M5] truncate body to 4096 + one-shot state clear; tests for both |
| 8 | `phase-08-admin-commands.md` | Key Insights, Grant subsection, Ban subsection, Impl Steps, Todo, Risk | [F5] audit-then-grant within pgx.Tx (rollback drops audit on grant failure); [M3] self-ban guard on `cfg.AdminTelegramIDs`; [L3] `parseAdminTelegramIDs` with fail-fast on non-int + warn+dedupe on dupes |
| 9 | `phase-09-message-templates.md` | User-facing key inventory, Admin-facing inventory, keys.go pseudocode | Added 5 new keys: `topup_overpaid_success` [Q1], `topup_recovered_late_payment` [Q2], `trial_phone_reused` [F4], `regen_rate_limited` [H5], `admin_self_ban_blocked` [M3] — TODO placeholders for copywriter |
| 10 | `phase-10-integration-tests.md` | Suite D (many new cases), Suite B (F4+H5), Suite F (F5+M3+L3), FireN helper (barrier-release), Stress Makefile+CI section (new), Related Code Files, Todo, Success Criteria, Next Steps, Risk | [F6] barrier-release FireN + stress matrix make target + CI step (100-iter × cpu=1,2,4,8); added test cases for every fix directive; [M6] real payload JSON fixture |

**Note:** phase-01 untouched except by reference from phase-06 (main.go admin channel wiring — called out in phase-06 Modify section).

## Migration delta summary (`20260424004_phase2_schema_deltas.sql`)

Applied after `20260424003_phase2_indexes.sql`. Goose NO TRANSACTION mode (ALTER TYPE ADD VALUE can't run in tx block).

```sql
-- Q1: bonus credit ledger event
ALTER TYPE ledger_event_type ADD VALUE IF NOT EXISTS 'topup_excess';
-- Q2: cancelled (user cancel retains provider_ref for recovery)
ALTER TYPE transaction_status ADD VALUE IF NOT EXISTS 'cancelled';
-- Q2: recovered_by_late_payment (late webhook on cancelled)
ALTER TYPE transaction_status ADD VALUE IF NOT EXISTS 'recovered_by_late_payment';
-- F2: drop unconditional UNIQUE, replace with partial UNIQUE on active states
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_provider_ref_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tx_provider_ref_active
  ON transactions(provider_ref)
  WHERE status IN ('pending','paid','cancelled','recovered_by_late_payment');
```

**Down migration limitations:** Postgres does not support removing enum values without recreating the type. Down only reverts the `provider_ref` constraint; enum values persist as unused after revert. Documented in migration comment + phase-04 + plan.md Risk Assessment.

## New risks introduced by patches

1. **Migration 004 NO TRANSACTION semantics** — if migration halts mid-way (e.g., DROP CONSTRAINT succeeded but CREATE INDEX failed), DB left in inconsistent state (no UNIQUE on provider_ref at all). Mitigation: idempotent DROP + idempotent CREATE (IF EXISTS / IF NOT EXISTS); `make migrate-up` retry completes safely.
2. **Over-payment bonus calc using `amount / (premiumCr+standardCr)` for combos** approximates per-credit rate. Combos (`p100_s50`, `p200_s100`) price premium and standard credits differently in reality, but the bonus calc uses a blended average. Integration test accepts this approximation; documented in webhook pseudocode comment.
3. **Redis `sepay_retry_queue` growth unbounded if consumer lag** — if lock contention persists, queue grows. Mitigation: consumer should TRIM queue to max 1000 entries; log alert when > 500. Deferred to phase-06 implementation detail during `/ck:cook`; not a blocker.
4. **`adminAlertCh` producers in audit thresholds (e.g., `sepay_auth_fail` 10-in-5min) not yet wired** — phase-08 mentions alert in risk table but implementation is admin-stats-side. This is an implementation detail for phase-08; the channel + consumer goroutine are ready in phase-06 plan.
5. **[Q2] Admin-initiated lifecycle for stuck `cancelled`** — no janitor job auto-expires cancelled rows in the active partial index. Stale rows could pile up if many users cancel without paying. Deferred to Phase 8 janitor (out of scope). Risk noted in phase-05 Risk table as "stale pending/cancelled rows."
6. **`SEPAY_BANK_ACCOUNT` env-name rename** — config.go changes required; old env var `SEPAY_BANK_ACC` deprecated. Must be in deploy checklist for Fly secrets.

## Unresolved questions

1. **Over-payment bonus pool for combos** — for `combo_p100_s50`, is excess credited to premium or standard? Plan assumes premium (rationale: combos always include premium as the valuable pool). Copywriter template `topup_overpaid_success` must render `BonusPool` field; template copy should be tone-flexible. Confirm with product before `/ck:cook` cook stage.
2. **SePay source IPs for allowlist** — requires docs research at deploy time. Plan-10 documents as pre-deploy blocker. Blocked until user/ops confirms from SePay docs or merchant dashboard.
3. **`WEBHOOK_RATE_LIMIT_PER_SEC` env var** — hard-coded 20 in plan. User may prefer env-configurable. Trivial to add during cook; defaulted to 20 in phase-06 Risk table.

## Plan file check

Frontmatter + structure preserved per `.claude/rules/documentation-management.md`. All patched files remain within schema (Context Links / Overview / Key Insights / Requirements / Architecture / Related Code Files / sqlc queries / Migration / Implementation Steps / Todo List / Success Criteria / Risk Assessment / Security Considerations / Next Steps).

## Commit ref

Pending — commit + push to `origin/dev` after report submitted.

**Status:** DONE
**Summary:** 10 plan files patched (9 phase files + plan.md) with 6 Q-decisions + 6 F-fixes + H5/H6/M3/M5/M6/L3. New migration `20260424004` spec added. 30+ new test cases specified across suites D, B, F. Stress matrix target added to Makefile + CI plan.
