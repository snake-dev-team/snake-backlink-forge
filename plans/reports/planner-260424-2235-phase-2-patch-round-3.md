---
title: "Planner Round 3 — Phase 2 Patch Report"
role: planner
date: 2026-04-24
phase: 2
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
status: DONE
round: 3
---

# Phase 2 — Patch Round 3

Surgical patches per user directives PART 1-6. No redesign, no scope expansion.

## Per-part completion table

| PART | Description | Files touched | Status |
|---|---|---|---|
| 1 | Domain deferral (Fly subdomain, Cloudflare Tunnel fallback, ngrok local dev) | `plan.md` (ADR + Risk row), `phase-06-sepay-webhook.md` (Security §Deploy target), `phase-10-integration-tests.md` (§Deploy target + checklist) | DONE |
| 2 | F5 Option A — grant_credits(ref_type='user', ref_id=target_uuid) → currval(ledger_id_seq) → audit_log.metadata.ledger_id | `phase-08-admin-commands.md` (Key Insights, §Grant requirements, Architecture pseudocode `AdminService.Grant`, Impl Steps, Todo, Risk, Success Criteria, Audit service callers note) | DONE |
| 3 | auditFailAlertWatcher goroutine (60s tick, 15min window, ≥20 threshold, bucket dedup) | `phase-08-admin-commands.md` (Key Insights, Architecture pseudocode, Impl Steps 6-7, Todo, Risk, Related Code Files Modify), `phase-06-sepay-webhook.md` (Risk row update), `phase-10-integration-tests.md` (Suite F `TestAuthFailBurstAlert`) | DONE |
| 4 | Retry queue — LTRIM 500 cap + BRPOP consumer + 3-retry dead-letter + 80% backlog alert | `phase-06-sepay-webhook.md` (§Retry queue + consumer section + pseudocode, Risk 3 rows, Impl Steps 7a-7c, Todo, Related Code Files Create + Modify), `phase-10-integration-tests.md` (Suite D 4 new tests + Todo entries) | DONE |
| 5 | TestAdminGrantAtomicRollback — 3 cases (grant fail, audit fail, happy with ledger_id match) | `phase-10-integration-tests.md` (Suite F detailed test spec with run command) | DONE |
| 6 | Low nits: uuid.uid→uuid.UUID; bonus var hoist; isPoolExhausted removed; migration 004 rephrase | `phase-06-sepay-webhook.md` (3 edits), `phase-04-wallet-ledger.md` (down migration note), `plan.md` (2 down-irreversibility rephrases) | DONE |

**Files modified (4):** `plan.md`, `phase-04-wallet-ledger.md`, `phase-06-sepay-webhook.md`, `phase-08-admin-commands.md`, `phase-10-integration-tests.md`.

## F5 atomicity proof (3 sentences)

Single pgx.Tx executes in order: grant_credits(ref_type='user', ref_id=target_user_id UUID) inserts ledger row and increments ledger_id_seq (step 2); `SELECT currval('ledger_id_seq')` reads the just-assigned id within the same session (step 3 — safe per Postgres docs); `INSERT audit_log` writes metadata.ledger_id=$bigint (step 4). Step 2 failure → no side effects, ROLLBACK is a no-op. Step 3 or 4 failure → ROLLBACK drops the ledger row + wallet delta atomically; COMMIT success guarantees both rows are present with cross-referencing ledger_id.

## Alert watcher dedup proof

The watcher computes `bucket := now.Unix() / (15 * 60)` — an integer division into 15-minute windows. The in-memory `alertedWindows map[int64]struct{}` records which buckets have already emitted an alert. On each tick: if `alertedWindows[bucket]` exists → skip entirely without querying DB. On threshold breach + not-yet-alerted → emit alert + `alertedWindows[bucket] = struct{}{}`. Garbage collection deletes buckets older than `bucket-4` (~1 hour) to prevent unbounded map growth while tolerating clock skew. A second breach within the same 15-min window produces zero alerts because the map entry short-circuits before the DB query.

## New risks introduced by round-3 patches

1. **Retry consumer uses BRPOP with no timeout (block forever):** correct by design (BRPOP 0 blocks until item or ctx cancel), but requires rootCtx cancel propagation on shutdown — covered in impl steps.
2. **`LPUSH` + `LTRIM` is 2 round-trips, not atomic:** between LPUSH and LTRIM, queue could briefly exceed 500. Acceptable — LTRIM on next producer call caps again; no correctness issue since queue is bounded-FIFO with LIFO eviction.
3. **`currval('ledger_id_seq')` assumes `grant_credits` always uses `DEFAULT` for ledger.id:** schema confirmed (init.sql:64 `id BIGSERIAL PRIMARY KEY`), and grant_credits proc INSERTs ledger without specifying id → `nextval` is guaranteed to be the proc's `INSERT`. If a future refactor uses explicit id, `currval` could read stale value. Documented in Risk row.
4. **`auditFailAlertWatcher` poll under DB load:** COUNT query on `audit_log` uses `idx_audit_user` index but scans all `event='sepay_auth_fail'` rows in 15-min window. At normal traffic (<100 auth fails expected in 15min outside an attack), cost is negligible. Under attack (100k+ auth fails), COUNT could be slow — acceptable since that's precisely the scenario we want an alert for.
5. **Test-only trigger for audit INSERT failure (PART 5 case 2):** requires `CREATE TRIGGER` per test. Trigger must be installed in test `SetUp` + dropped in `TearDown`. If test harness fails mid-test, trigger leaks — testcontainers container teardown handles via DB drop. Documented in test spec.
6. **Retry envelope JSON unmarshal failure → silent drop:** consumer logs and continues on bad envelope (no re-enqueue). Acceptable since bad envelope is unrecoverable.

## Residual from round 2 not closed (intentionally deferred)

- **`maxRate(txRow)` helper undefined:** no longer referenced (replaced with hoisted `bonus` var in PART 6).
- **`isPoolExhausted(err)` helper:** removed from pseudocode (PART 6) — only `errors.Is(err, pgx.ErrAcquireTimeout)` remains for pool-exhausted detection.

## Plan file format check

All touched files preserve existing section schema per `.claude/rules/documentation-management.md`. `plan.md` frontmatter unchanged. No new phase files created.

## Unresolved questions

1. **SePay `.fly.dev` acceptance** — user verifying with SePay dashboard; fallback to Cloudflare Tunnel documented but not configured. Blocks prod deploy smoke test only, not Phase 2 planning.
2. **Retry consumer alert dedup flag persistence** — flag `backlogAlertedHigh` lives in consumer goroutine memory. On process restart, flag resets → first-tick alert if queue still >400. Acceptable (operator should already know about backlog from previous alert).
3. **`audit_log.metadata` JSONB size limit with large metadata:** for `admin_grant`, metadata is small (~200 bytes). Not a concern for Phase 2.
4. **Test trigger isolation for PART 5 case 2** — if multiple test suites run in parallel against same DB container, trigger could interfere. Mitigation: testcontainers spins fresh PG per suite (existing behavior); document in test comment.

## Commit ref

Pending — commit + push after report.

**Status:** DONE
**Summary:** 5 plan files patched with PART 1-6 directives. F5 rewritten to Option A (grant first, currval capture, audit with ledger_id in metadata). auditFailAlertWatcher wired in phase-08 implementation. Retry queue bounded + consumer with 3-retry dead-letter + 80% backlog alert. 8 new tests added across Suites D + F. Low nits cleaned (uuid.UUID, bonus hoisting, isPoolExhausted removed, migration 004 rephrased).
