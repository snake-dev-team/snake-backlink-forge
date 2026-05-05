---
title: "Red-Team Review Round 2 — Phase 2 Plan Patches"
role: code-reviewer (red-team round 2)
date: 2026-04-24
phase: 2
round: 2
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
commit: 1f67020
status: FIX_REQUIRED
---

# Red-Team Review Round 2 — Phase 2 Plan

## Verdict

**FIX_REQUIRED** (one new critical, otherwise patches largely effective). F1/F2/F3/F4/F6 and H5/M3/M5/M6/L3/Q1-Q6 all genuinely close. **F5 patch introduces a regression: the `audit_log.id` is `BIGSERIAL`, but `grant_credits(..., p_ref_id UUID)` requires UUID — passing `audit_id` as `ref_id` will fail with Postgres type cast error.** Additionally, planner's new-risk #4 (`adminAlertCh` producer for `sepay_auth_fail` 10-in-5min) and over-payment `BonusCredits` final-return line (`max0(int(diff / maxRate(txRow)))` references undefined helpers) remain unresolved. Fix F5 + wire the auth-fail alert producer before cook, then APPROVED.

Minor quality nits (pseudocode typos, unused helpers) will be caught by normal review during `/ck:cook`; they are not cook-blockers if core ordering is correct.

---

## Round 1 critical finding closure audit

### F1 — Over-payment 3-way branch
- **Status:** CLOSED_WITH_OBS
- **Verification:** `phase-06-sepay-webhook.md:45-48` (requirements) + `:246-293` (code). Switch on `diff` is `<0 | ==0 | >0`, each branch handled. Equal case falls through to base grants (correct, no ambiguity with overpaid).
- **Attack outcomes:**
  - Replay of same overpaid webhook: first sets status=paid, second CAS sees `status='paid' ∉ ('pending','cancelled')` → ErrNoRows → out-of-band lookup returns `AlreadyProcessed=true`. ✓ no double-credit.
  - `transferAmount == amount_vnd + 1đ`: rate ≈ 1645, bonus = 1/1645 = 0. Audit `sepay_overpaid` fires with `bonus=0, diff=1` → benign log pollution (1đ noise). Below 10k threshold, no admin alert. Acceptable.
  - Over-payment for combo pkg (`combo_p100_s50`): `perCreditRate` computes `rate := amount / (premiumCr+standardCr)` = blended average, not true premium rate. Planner new-risk #2 disclosed. Pool hardcoded to `"premium"` for combos. Documented in code comment. Accepted approximation.
- **New observations:**
  - `ProcessResult{...BonusCredits: max0(int(diff / maxRate(txRow)))}` on line 312 references `max0()` and `maxRate()` helpers not defined anywhere in the plan. This line also re-derives BonusCredits from scratch instead of using the `bonus` local variable computed earlier in the overpaid branch — **inconsistent**. Minor pseudocode bug; will surface at cook.
  - `UserID uuid.uid` typo at line 181 (should be `uuid.UUID`). Pseudocode.
- **Risk:** Negligible; pseudocode correctness caught at implementation review.

### F2 — 12-hex order code + partial unique
- **Status:** CLOSED_WITH_OBS
- **Verification:**
  - Gen: `phase-05-transactions-topup.md:79-81` — `u := uuid.New()` → `hex.EncodeToString(u[:6])` → `strings.ToUpper(...)` = 12 hex chars uppercase. ✓ (`uuid.New()` returns `uuid.UUID` = `[16]byte`; slicing works.)
  - Query normalization: `phase-06:105` — `orderCode := strings.ToUpper(match[1])` before SQL. ✓
  - Regex: `phase-06:335` — `(?i)SBF\s+TOPUP\s+([A-F0-9]{12})`. Case-insensitive capture (matches lower+mixed memos), output normalized via ToUpper. ✓
  - Partial unique: `phase-04:133-135` — `idx_tx_provider_ref_active` on `status IN ('pending','paid','cancelled','recovered_by_late_payment')`. Includes cancelled (needed for Q2 recovery). ✓
  - Retry loop: `phase-05:79-110` — 3x attempts on 23505/`idx_tx_provider_ref_active`; fresh UUID each loop. ✓
- **Attack outcomes:**
  - Partial index excludes `manual_review`. If a tx is manual-reviewed with `provider_ref=X`, a new pending tx may reuse the same provider_ref. Risk: second tx's webhook could match the old provider_ref. BUT partial unique already requires active-state uniqueness; a new pending is itself in active state → first one must not still be active. If first tx is `manual_review`, second pending with fresh (different) provider_ref is generated. Only collision vector is if migration 004 insert race produces `manual_review` + `pending` sharing a code — negligible 48-bit probability.
  - Migration 004 NO TRANSACTION gap: `DROP CONSTRAINT` → (gap) → `CREATE UNIQUE INDEX`. Concurrent INSERT with duplicate provider_ref during the <100ms gap could succeed. Planner flagged this (new-risk #1). Mitigation: `IF EXISTS`/`IF NOT EXISTS` idempotency + run during deploy downtime. **Accepted with deploy-note constraint.**
- **New observations:** None cook-blocking.
- **Risk:** Migration 004 MUST run during a window with no topup insert traffic. Add to deploy checklist.

### F3 — Error classifier
- **Status:** CLOSED_WITH_OBS
- **Verification:** `phase-06:132-158` — explicit switch on `pgerrcode.DeadlockDetected`, `pgerrcode.SerializationFailure`, `context.DeadlineExceeded` → 200 queued_for_retry + Redis LPush `sepay_retry_queue`. `pgx.ErrAcquireTimeout` + pool-exhausted → 503. Default → 500 + audit + admin alert. ✓
- **Attack outcomes:**
  - Retry queue unbounded: planner admitted in new-risk #3, punted cap=1000 to cook-stage. Not ideal but accepted for Phase 2 scale.
  - `isPoolExhausted(err)` helper not defined in plan. Implementation detail.
  - Retry consumer goroutine not specified (no backoff, no max-retries-per-message, no idempotency-across-retries proof beyond existing CAS — which is adequate, but lack of dead-letter path means a permanently-failing message loops). Partially disclosed in Risk table Phase 2 scope.
  - `sepay_auth_fail` burst alert: planner new-risk #4 confirmed NOT WIRED. Phase-06 Risk row (line 465) says "Alert on 10+ consecutive via adminAlertCh — goroutine DMs admin (Phase 08)" but **phase-08 implementation steps + todo list have ZERO mention of this goroutine**. Only admin channel infrastructure (Q5) exists; no producer. **Gap remains from round 1 (was originally M7). Must wire producer in phase-08 implementation steps.**
- **New observations:** See F3 residual gap for auth-fail alert producer.
- **Risk:** Medium — without auth-fail alert, token rotation mishap (SePay rotates key silently) results in ALL webhooks silently 200 success=false. Detection latency = admin noticing low revenue.

### F4 — Atomic trial gate
- **Status:** CLOSED
- **Verification:** `phase-02:72-112` — `BEGIN → SELECT FOR UPDATE → conditional UPDATE ... WHERE trial_used=FALSE (partial-index-catches-23505) → grant → COMMIT`. No COUNT(*) pre-check. ✓
- **Attack outcomes:**
  - Two tg_ids + same phone, concurrent: each flows through its own row (different tg_id → different user row → different FOR UPDATE lock target). Both issue `UPDATE SET phone_e164=X, trial_used=TRUE`. Partial unique `idx_users_phone_trial WHERE trial_used=TRUE` guards: first commits, second gets 23505 on UPDATE → caught as `ErrTrialPhoneReused`. ✓
  - Double-tap same tg_id: singleflight removed per plan update, but `SELECT FOR UPDATE` on same row serializes the second caller behind the first. After first commits (`trial_used=TRUE`), second's `UPDATE ... WHERE trial_used=FALSE` affects 0 rows → `ErrTrialAlreadyUsed`. ✓
  - Existing user with `trial_used=FALSE` (e.g., `/start ref_X` before phone share): `UPDATE` flips trial_used=TRUE atomically in same statement. Handled.
  - Concurrent same phone, different tg_ids, **23505 error**: Phase-02 line 94-96 catches `pgErr.Code == "23505"` AND constraint name/message contains "phone"  → `ErrTrialPhoneReused`. Note: `strings.Contains(pgErr.Message, "phone")` is a fallback for non-PG12 driver variants where `ConstraintName` may be empty. Pragmatic. ✓
  - Phase-10 Suite B concurrent phone test present (phase-10:34). ✓
- **New observations:** None.

### F5 — Audit before ledger
- **Status:** STILL_OPEN — new critical regression
- **Verification:** `phase-08-admin-commands.md:37-42` — documented order: BEGIN → INSERT audit_log RETURNING id → grant_credits(..., `ref_id=$audit_id`) → COMMIT. Ordering is correct.
- **NEW CRITICAL BUG:**
  - `audit_log.id` is **BIGSERIAL** (init.sql:250) → int8/bigint.
  - `grant_credits(p_ref_id UUID)` (init.sql:318) — parameter typed **UUID**.
  - `ledger.ref_entity_id` is **UUID** (init.sql:71).
  - Passing `$audit_id` (bigint) to a UUID parameter **will fail with a Postgres cast error** (`invalid input syntax for type uuid`).
  - Plan says `event='admin_grant_credits', ref_type='audit_log', ref_id=$audit_id` — type mismatch blows up at runtime.
- **Attack outcomes:**
  - Every `/admin grant` would crash — unable to grant any admin credits until fixed.
  - Test case `phase-08:199` claims "grant_credits failure" triggers rollback + audit absent — but the grant call itself errors on type cast BEFORE any money flow. Integration test would expose immediately.
- **Fix required:**
  - **Option A (preferred)**: Keep `ref_type='user', ref_id=target_user_id` (UUID) on ledger, and put `audit_id` in metadata. The audit row itself references the ledger via separate `ref_type='ledger_id_bigint_in_metadata'` or by storing `{"ledger_id": <id>}` in audit_log.metadata after the grant returns.
  - **Option B**: Alter `ledger.ref_entity_id` column to TEXT or add separate `audit_ref_id BIGINT` column. Invasive; requires migration, breaks sqlc generated code.
  - **Option C (minimal)**: Run audit_log INSERT first, receive bigint id, immediately do `gen_random_uuid()` for the ledger ref and store the pair `{"ledger_uuid": X, "audit_id": bigint}` in both audit_log.metadata and ledger.metadata; ref_type='admin_cross_ref'.
  - **Cleanest for Phase 2:** Use `ref_type='user', ref_id=target_user_id` for the ledger row (as already done for trial_grant in phase-02). Insert audit_log AFTER grant_credits within the SAME tx; capture `grant_credits`' returned ledger id from a RETURNING-style wrapper (stored proc currently returns `new_balance` only, not ledger_id — either extend wrapper or do `SELECT currval('ledger_id_seq')` inside same tx, which is safe within a session).
  - Then audit_log.metadata can record `{"target_user_id": uuid, "ledger_id": bigint, "pool": ..., "amount": ...}` — chain-of-evidence complete without type collision.
- **Risk:** HIGH. **Blocks admin grant feature entirely.** Must fix before cook.

### F6 — Race test rigor
- **Status:** CLOSED
- **Verification:**
  - `FireN` barrier-release: `phase-10:144-163` — `chan struct{}` + `wg` + `close(start)` simultaneously releases all N goroutines. 2ms park before release to ensure all goroutines reach the barrier. Superior to `errgroup.Go` sequential dispatch. ✓
  - Makefile: `phase-10:169-171` — `test-integration-stress: go test -race -count=100 -cpu=1,2,4,8 -run 'TestWebhookRace|TestTrialRace|TestTopupIdempotency' ./internal/e2e/...`. Exact match to mandate. ✓
  - CI step: `phase-10:174-176` — `- name: Stress race tests; run: make -C services/api test-integration-stress`. ✓
  - Success criteria explicit: `phase-10:247` — "100/100 iterations pass across -cpu=1,2,4,8 matrix". ✓
  - Related Code Files includes `.github/workflows/ci.yml` (line 199). ✓
  - Suite D test cases count: over-payment exact + 50k flag + admin alert (3), cancel-then-pay (1), account_mismatch (1), gateway_mismatch (1), rate_limit (1), mixed-case memo (1), deadlock_injection (1), real_payload_fixture (1), concurrent 10x (1), underpayment (1), idempotent_replay (1), wrong apikey (1), Bearer scheme (1), unmatched content (1), transferType=out (1). **17+ new/confirmed cases** in Suite D. Exceeds round 1 ask.
- **Attack outcomes:**
  - Barrier-release eliminates sequential dispatch jitter.
  - 2ms sleep before `close(start)` is a runtime-dependent fudge; in theory stragglers could still miss the release window. Pragmatic.
- **New observations:** None.

---

## Round 1 H/M findings closure

| Finding | Status | Verification |
|---|---|---|
| H1 account number match | CLOSED (Q3) | phase-06:90-93, env `SEPAY_BANK_ACCOUNT` + `SEPAY_BANK_CODE` required. Strict match + audit on mismatch. |
| H2 rate limit on webhook | CLOSED (Q4) | phase-06 middleware `rate_limit_webhook.go`, Redis INCR fixed window 20/s/IP → 429. Pre-deploy IP allowlist flagged. |
| H3 case sensitivity | CLOSED (F2) | `strings.ToUpper(match[1])` at phase-06:105. |
| H5 regenkey rate limit | CLOSED | phase-03:26, 116, 131. Redis `regen_rl:<uid>` INCR + EXPIRE 86400; short-circuit before FSM. Integration test at phase-03:123. |
| H6 notify goroutine context | CLOSED | phase-06:122-126 — `context.WithTimeout(deps.RootCtx, 10*time.Second)` + defer cancel. No more `context.Background()`. |
| H7 cancel-then-pay | CLOSED (Q2) | phase-05:38, 163 — CancelPendingTransaction sets `status='cancelled'`. phase-06:210-225 CAS widened to include `cancelled` → transitions to `recovered_by_late_payment`. Template + test present. |
| M1 /campaigns | CLOSED (Q6 scope-cut) | plan.md:67-69, phase-07:33. Removed from Phase 2 inventory (12 commands). Deferred to Phase 4+. No router entry. |
| M3 self-ban | CLOSED | phase-08:17, 46, 191, 201, 236. Guard checks `cfg.AdminTelegramIDs`. Template + test. |
| M5 body cap + one-shot | CLOSED | phase-07:29, 143 — truncate 4096 + state.Clear() before reply. Two tests at phase-07:151-152. |
| M6 real payload fixture | CLOSED | phase-10:187, 228 — `testutil/sepay_payload_real.json`. Suite D asserts parse. |
| L3 admin IDs parsing | CLOSED | phase-08:18, 187, 203-204. Comma-separated, fail on non-int, warn+dedupe. |
| M7 (from round 1 #7) auth-fail alert goroutine | STILL_OPEN | Planner new-risk #4. Phase-06 Risk row mentions, phase-08 implementation DOES NOT have explicit alert producer. See F3 residual. |
| M4 ip_hash length | CLOSED (noted) | phase-06:484 — full 32-char hex. |
| M2 package regex over-permissive | Not addressed | Minor. Regex allows invalid combos like `premium_starter_300` (not in map). Defense-in-depth fine; map lookup rejects. Not a blocker. |

---

## New findings introduced by patches (round 2)

### [C-Round2-1] F5 audit→ledger type collision (BLOCKER)
See F5 section above. `audit_log.id BIGSERIAL` cannot fit `grant_credits(p_ref_id UUID)`.

### [H-Round2-1] adminAlertCh auth-fail producer not wired
- **Location:** phase-08 implementation steps (lines 184-204) do NOT include any producer for `sepay_auth_fail` 10-in-5min burst. Only admin-initiated commands are covered.
- **Impact:** Round 1 M7 remains open. SePay token rotation → silent payment loss.
- **Fix:** Add implementation step to phase-08:
  - Spawn `auditFailAlertWatcher(ctx, db, adminAlertCh)` goroutine in main.go startup. Every 60s, `SELECT COUNT(*) FROM audit_log WHERE event='sepay_auth_fail' AND created_at > NOW() - INTERVAL '5 minutes'`. If ≥ 10 → `sendAdminAlertNonBlocking(AdminAlert{Kind: "auth_fail_burst", ...})`.
  - Add to phase-08 Todo List + Success Criteria.
  - Add to phase-10 Suite F: "burst 10x auth_fail → admin alert received on channel within 60s".

### [L-Round2-1] Pseudocode typos (non-blocking)
- `uuid.uid` in `ProcessResult.UserID` declaration (phase-06:181) — should be `uuid.UUID`.
- `max0(int(diff / maxRate(txRow)))` on phase-06:312 — `max0` and `maxRate` helpers undefined. The `bonus` local var from line 267 is the computed value; the return-line re-derivation is dead code / inconsistent.
- `isPoolExhausted(err)` on phase-06:147 — helper undefined.
- These are pseudocode-level; will surface at cook. Not cook-blockers but recommend fixing in plan so cook doesn't invent diverging helpers.

### [L-Round2-2] Retry queue cap + consumer undefined
- Redis `sepay_retry_queue` unbounded; planner punted cap=1000 to cook. Consumer goroutine / backoff / max-retries-per-message not speced.
- Impact: If deadlocks cluster, queue grows; if consumer loops a permanently-poisoned message, CPU burn. Accept as cook-stage detail but document as Phase 2 OUT-of-scope Risk (not already in phase-06 Risk table — add there).

### [L-Round2-3] Migration 004 irreversibility understated
- Postgres CAN remove enum values via CREATE TYPE + swap (destructive). Plan says "cannot" which is strictly false; more accurate: "practically irreversible without data migration". Minor doc nit.

---

## New attack scenarios result

### N1 — Over-payment amount DoS (repeated 1đ over-pay)
- **Outcome:** Every webhook audits `sepay_overpaid` with `bonus=0, diff=1`. Log pollution. No admin alert (below 10k). No money-flow corruption.
- **Risk:** Minor log noise. Acceptable. If a real attacker runs this, they LOSE 1đ per replay to us. Economic disincentive.
- **Status:** Accepted residual risk.

### N2 — Retry queue poisoning via forged webhook
- **Outcome:** Rate limit (Q4) 20/s/IP gates webhook entry BEFORE auth. Attacker without IP control can't pump Redis queue fast. If attacker controls an IP, forges Apikey-valid webhook, then induces DB deadlock → payload lands in queue. BUT to induce deadlock, attacker must already own valid Apikey + trigger concurrent tx on target row — which requires them to ALSO own a legit pending order. In that case, they're their own victim.
- **Risk:** Extremely low. Accepted.
- **Status:** Accepted.

### N3 — `recovered_by_late_payment` race during user cancel
- **Scenario:** User clicks "cancel" mid-webhook.
- **Outcome:** Webhook `BEGIN TX + UPDATE WITH target AS (SELECT ... WHERE status IN ('pending','cancelled') FOR UPDATE)` row-locks. Cancel's `UPDATE transactions SET status='cancelled' WHERE id=X AND status='pending'` either: (a) runs first, webhook sees `cancelled` → transitions to `recovered_by_late_payment`; or (b) blocks on webhook's FOR UPDATE, runs after webhook commits → sees `status='paid' OR 'recovered_by_late_payment'` → RowsAffected=0, cancel no-op. Either way consistent. Cancel's `WHERE status='pending'` clause is the gate.
- **Status:** Closed by row-lock + conditional WHERE. No lost recovery.

### N4 — Trial gate ON CONFLICT surrogate for partial unique
- **Observation:** Plan does NOT use `INSERT ... ON CONFLICT`. It uses `SELECT FOR UPDATE + conditional UPDATE`. This is CORRECT for the existing-row case; upstream loadUser creates the user row.
- **Edge:** if loadUser middleware fails to create user row (e.g., DB hiccup), trial gate's `SELECT ... WHERE id=$1` returns no rows → bubble up as generic DB error, not `TrialPhoneReused`. Handled acceptably. ✓

---

## Net verdict for /ck:cook readiness

**RED** on F5 (type collision crashes every `/admin grant`).
**YELLOW** on H-Round2-1 (auth-fail alert unwired — tolerable short-term but a silent failure mode).
**GREEN** on everything else.

Cook is blocked until F5 is rewritten. After that, auth-fail alert wiring is recommended before production but could theoretically ship to dev for initial validation.

---

## Recommended pre-cook actions (delta from round 1)

1. **[BLOCKER] Rewrite F5 in phase-08 to avoid UUID/BIGSERIAL type collision.**
   - Preferred: grant_credits called with `ref_type='user', ref_id=target_user_id`; audit_log INSERTed after grant_credits within the SAME tx; audit_log.metadata contains `{"ledger_id": <returned_bigint>, "target_user_id": <uuid>, "pool": ..., "amount": ..., "reason": ...}`. Chain-of-evidence: ledger → audit via audit_log.metadata.ledger_id; audit → ledger via ledger.created_at + user_id fuzzy match. Acceptable for audit purpose; integrity preserved by atomic COMMIT.
   - Tx order within the same pgx.Tx: (1) `grant_credits` returns new_balance (stored proc also inserts ledger row; capture id via `SELECT currval('ledger_id_seq')`), (2) INSERT audit_log with ledger_id in metadata, (3) COMMIT. On grant failure → ROLLBACK drops both.
   - Update phase-08 lines 37-42 + 191 + 199 + success criteria accordingly.
2. **[HIGH] Wire `sepay_auth_fail` burst alert producer in phase-08 implementation steps + todo list.**
   - Add goroutine spec: `go auditFailAlertWatcher(rootCtx, db, adminAlertCh)`. Polls every 60s; threshold 10-in-5min.
   - Add integration test to Suite F.
3. **[LOW, optional] Clean up F1 pseudocode typos.**
   - Fix `uuid.uid` → `uuid.UUID` at phase-06:181.
   - Replace `max0(int(diff / maxRate(txRow)))` with the already-computed `bonus` local variable.
   - Define `isPoolExhausted(err)` helper OR replace with explicit `errors.Is(err, pgx.ErrAcquireTimeout)` check only.
4. **[LOW] Add retry queue cap + consumer contract to phase-06.**
   - Redis LPUSH with LTRIM to 1000.
   - Define consumer loop: `RPOP sepay_retry_queue` → replay ProcessPaidTransaction; on success → delete; on failure after 3 retries → dead-letter key `sepay_dead_letter` + admin alert.
5. **[LOW] Migration 004 deploy-note:** add to plan.md Risk Assessment: "Apply during low-traffic window; DROP→CREATE gap is non-atomic."

---

## Residual risks accepted (updated list)

1. **Combo bonus blended per-credit rate** — approximation; accepted per planner new-risk #2.
2. **Migration 004 NO TRANSACTION DROP→CREATE gap** — ~100ms window without unique guard; deploy during downtime.
3. **Retry queue unbounded (no cap in plan)** — deferred to cook; add LTRIM 1000 as cook-stage detail.
4. **1đ over-pay log noise** — benign.
5. **sepay_auth_fail alert unwired** — HIGH risk, see H-Round2-1. Should be closed pre-cook, but technically non-blocking if monitored manually during first production week.
6. **Down-migration of enum values** — effectively irreversible; accepted.
7. **Admin chat has no IP context** — metadata `source="telegram_admin"` substitute.

---

## Unresolved questions

1. **F5 fix preference:** Option A (ref_type='user', ledger_id in audit metadata) vs Option C (gen_random_uuid + cross-ref in both metadata fields)? User/planner decides — A is simpler, C preserves stricter typing in audit_log.
2. **sepay_auth_fail threshold:** 10-in-5min is inherited from round 1 discussion. Confirm or adjust (e.g., 20-in-15min less noisy).
3. **Retry queue cap value:** 1000 is an opinion; confirm or adjust (maybe 500 given expected <2 webhooks/sec from SePay).
4. **Whether to add rollback-test for F5** in phase-10 Suite F — yes recommended (integration test: inject stored-proc failure mid-grant → assert both audit_log and ledger rows absent after rollback).

---

**Status:** DONE_WITH_CONCERNS
**Verdict:** FIX_REQUIRED
**Summary:** F1/F2/F3/F4/F6 closed (5/6 critical). **F5 REGRESSION**: audit_log.id BIGSERIAL vs grant_credits.p_ref_id UUID type collision blocks admin grant entirely. H5/M3/M5/M6/L3/Q1-Q6 closed (11/11). New findings: 1 critical (F5 redo), 1 high (auth-fail alert unwired — round 1 M7 incomplete), 3 low (pseudocode typos, retry queue cap, migration doc nit).
**Report:** E:\tool_backlink\plans\reports\redteam-260424-2209-phase-2-round-2.md
**Commit sha:** 6826b58
**Top blockers for cook:**
1. F5 type-collision rewrite (phase-08 admin grant tx)
2. auth-fail alert producer wiring (phase-08 impl steps)
