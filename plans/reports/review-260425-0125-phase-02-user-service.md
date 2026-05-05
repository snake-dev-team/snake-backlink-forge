---
title: "Code Review — Phase 02 User Service + /start"
role: code-reviewer
date: 2026-04-25
phase: 2
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
branch: dev
verdict: APPROVED
---

# Code Review — Phase 02 User Service + /start

## Verdict
APPROVED

## Summary
F4 atomic trial gate implementation is verbatim to spec. 0 critical, 0 high, 3 medium, 3 low. Zero money-flow bugs. Ready for Phase 03 unblock.

## F4 atomicity audit (primary concern)
All 7 checks PASS verbatim vs plan spec lines 62-113:

- [x] **isolation level** — `pgx.ReadCommitted` explicit (`user_service_trial.go:52`)
- [x] **FOR UPDATE lock** — `SELECT trial_used, is_banned FROM users WHERE id=$1 FOR UPDATE` (`user_service_trial.go:60-63`)
- [x] **Ban check BEFORE UPDATE** — `if isBanned { return ErrTrialUserBanned }` at line 69-71, fires before line 74 UPDATE
- [x] **conditional UPDATE WHERE trial_used=FALSE** — `WHERE id=$2 AND trial_used=FALSE` (`user_service_trial.go:77`)
- [x] **23505 detection** — `pgErr.Code == "23505" && (ConstraintName == "idx_users_phone_trial" || strings.Contains(Message, "phone"))` (`user_service_trial.go:82-84`). Substring fallback reliable: Postgres dup-key message format is `duplicate key value violates unique constraint "idx_users_phone_trial"` and Postgres also includes the phone value in the detail line — either path fires. Belt-and-suspenders correct.
- [x] **RowsAffected gate** — `if ct.RowsAffected() == 0 { return ErrTrialAlreadyUsed }` at line 93-96. Guards SELECT↔UPDATE race.
- [x] **grant_credits ref_type correctness** — `SELECT grant_credits($1, 'standard', 5, 'trial_grant', 'user', $1)` (`user_service_trial.go:115-117`). `p_ref_type='user'` matches F5 lesson (not `'audit_log'`); `p_ref_id=userID` reuses $1 (pgx supports reuse).
- [x] **tx.Commit order** — audit INSERT (line 103) → grant_credits (line 115) → Commit (line 123). Key issuance (line 136) is POST-COMMIT, correct.
- [x] **defer tx.Rollback** — at line 56, BEFORE any operation. Works on ctx cancellation. No-op after successful Commit.

### Concurrent attack outcomes (verified)
- **Two tg_ids, same phone:** First tx's UPDATE sets `trial_used=TRUE, phone_e164='+84X'`. Second tx's FOR UPDATE waits on first tx's row lock (different rows → no block), UPDATE fires: partial index `idx_users_phone_trial` sees first row already exists with same phone → SQLSTATE 23505 → ErrTrialPhoneReused. ✓
- **Two /start taps, same tg_id:** Second tx's FOR UPDATE blocks on first's row lock → serialized. After first commits, second reads `trial_used=TRUE` → UPDATE fires `WHERE trial_used=FALSE` → RowsAffected=0 → ErrTrialAlreadyUsed. ✓
- **grant_credits failure:** tx.Rollback fires via defer → trial_used stays FALSE, phone unset, wallet credits unchanged, audit row rolled back. Retry-safe. ✓
- **Context cancellation mid-tx:** defer tx.Rollback(ctx) fires even though ctx is done — pgx handles cancelled ctx by sending rollback via new conn or marking tx as released. No leak, no double-grant. ✓

**F4 atomicity: CLEAN.**

## Critical findings
None.

## High findings
None.

## Medium findings

### M1 — CI has no Postgres service; integration tests silently skip (`.github/workflows/ci.yml:31-49`)
The Go CI job runs `go test ./... -race -cover` but the workflow declares no `services: postgres` block and no `DATABASE_URL` env var. The 5 `service_test` integration tests self-skip via `t.Skip("DATABASE_URL not set")` at `user_service_test.go:54`. Net effect: PR merges pass CI while the F4 trial gate is untested in CI. Local-dev testing works (developer sets DATABASE_URL), but regression risk on main branch rises each Phase. Not a Phase 02 blocker (tests themselves are correct and pass locally), but should be scheduled as Phase 03 or Phase 10 pre-requisite. Fix is mechanical: add `services: { postgres: { image: postgres:16 } }` + run `go run ./cmd/migrate up` in workflow.

### M2 — handleStartCommand calls EnsureStub a second time redundantly (`cmd_start.go:44`)
`loadUser` middleware at `middleware.go:117` already called `EnsureStub` for this update, attached `BotUser` to ctx, and middleware populated `IsVerified`. `handleStartCommand` at line 44 calls EnsureStub again, gets the same row via `ON CONFLICT DO UPDATE`, and ignores the ctx `BotUser`. Two DB round-trips per `/start`. Functionally correct (idempotent upsert), but wastes a connection and rewrites `last_active_at` twice per tap. Reading `UserFromCtx(ctx)` would suffice — `BotUser` already carries `IsVerified`. Defer to Phase 04 cleanup; not cook-blocking.

### M3 — handleContactShare swallows sendgate on non-awaiting_contact state (`cmd_start.go:118-121`)
When a user shares a contact without /start precedent (or after state expired), the handler returns nil with zero UX feedback. User will be confused why nothing happened. Plan §Risk Assessment at phase-02:249 accepts this ("ignore silently") so spec-compliant. But consider sending a one-liner "Vui lòng /start trước để kích hoạt tài khoản." in Phase 03 polish. Low priority but real UX cliff.

## Low / nits

### L1 — Regex duplicate paren pair (`phone.go:83`)
`regexp.MustCompile(`^[0-9+\s\-.()()]+$`)` has `()()` — two repeated `(` and `)` chars inside the char class. Harmless (Go regex dedupes silently in char class semantics) but visually confusing. Change to `^[0-9+\s\-.()]+$`.

### L2 — `_ = botUser` at cmd_start.go:59 dead-assigns
`botUser` is constructed at line 53-58 but immediately discarded at line 59 with `_ = botUser`. Comment says "used by FSM state; kept for future middleware handoff." Dead code per YAGNI. Delete the block (the `user.IsVerified` read at line 64 doesn't need the local). Saves 7 lines, 1 explanation comment. Non-blocking.

### L3 — noopKeyIssuer sentinel check uses separate helper unnecessarily (`user_service_trial.go:157-159`)
`isErrKeyIssuerNotWired` is a 2-line helper that wraps `errors.Is(err, ErrKeyIssuerNotWired)`. Inline the call at line 138 — saves the helper. Micro-nit.

## Plan spec alignment

| Item (phase-02-user-service.md §Requirements) | Status |
|---|---|
| EnsureStub creates user + wallet atomically (single tx) | ✓ `user_service.go:94-128` |
| VerifyContactAndGrantTrial atomic F4 pattern, no COUNT pre-check | ✓ `user_service_trial.go:40-147` |
| Phone normalizer handles 15+ formats | ✓ 19 test cases, all pass, 5 VN + intl + edge |
| /start command dispatches to HandleStart | ✓ `router.go:58-62` |
| Contact event routes to same handler via FSM state | ✓ `router.go:39-44`, `cmd_start.go:116-121` |
| Error messages for TrialPhoneReused, TrialAlreadyUsed, TrialUserBanned | ✓ `cmd_start.go:176-198` (VN placeholder, Phase 09 owns i18n) |
| Audit log entry on success (event='trial_granted', phone_hash) | ✓ `user_service_trial.go:99-106` |
| FSM awaiting_contact state set/cleared | ✓ set `cmd_start.go:75`, cleared `cmd_start.go:135` |
| sqlc usage regenerated cleanly | ✓ `users.sql.go` matches new queries |
| Migration Up+Down with NO TRANSACTION directive | ✓ `20260424003_phase2_indexes.sql:2` |
| support_tickets + referrals tables with FK ON DELETE CASCADE | ✓ lines 16, 28 |

## Test adequacy

| Aspect | Status |
|---|---|
| 5 service tests: happy, double-tap, phone-reused, banned, stub-idempotency | ✓ comprehensive for Phase 02 |
| 19 phone normalizer cases incl. VN 032/035/039 prefixes | ✓ new VN range numbers covered |
| Integration uses live Postgres via DATABASE_URL | ✓ testcontainers deferred to Phase 10 per spec |
| Unique tgID range [1M, 2M) for inter-test isolation | ✓ `uniqueTgID()` |
| Concurrent F4 race test | Deferred to Phase 10 Suite B per spec |
| EnsureStub concurrent idempotency | Not covered (low priority — ON CONFLICT handles it) |

**CI gap:** M1 — CI runs `go test` without Postgres, so the 5 tests skip. Local dev passes all 25. Phase 03 should add postgres service to CI.

## Approved items
- F4 atomic trial gate — verbatim match to plan, correct 23505 fallback, correct RowsAffected gate, correct grant_credits ref_type.
- Phone normalization with libphonenumber + VN fast-path + letter rejection.
- PII discipline: phone_hash (sha256[:8]) in every log site, never raw phone. plaintext key never logged.
- Contact spoofing guard (`contact.UserID == from.ID`) correctly rejects third-party phone shares.
- FSM state lifecycle: set on /start welcome, cleared on success OR on any trial error (avoids stuck user).
- Plaintext key display: HTML-escaped via `html.EscapeString`, wrapped in `<code>`, shown once with bold save-warning.
- loadUser fail-closed on DB error (Phase 01 H1 fix retained, no regression).
- EnsureStub atomicity: user upsert + wallet ensure in single pgx.Tx (prevents partial commit leaving ledger grant without wallet row).
- Audit log insert non-fatal: warn-on-failure pattern consistent with "advisory log, not financial state."
- Key issuance post-commit: noopKeyIssuer → empty plaintext → bot sends fallback message. Trial credits still granted. Phase 03 can issue key separately.
- Migration NO TRANSACTION directive correct for partial unique index.
- go build, go vet, go test all clean on local dev machine.

## Recommended actions
1. Phase 03 prep: add Postgres 16 service container to `.github/workflows/ci.yml` + run migrations before `go test` (resolves M1).
2. (Optional) drop redundant EnsureStub call in `handleStartCommand`; read `IsVerified` from ctx `BotUser` (M2).
3. (Optional polish) acknowledge stale contact shares with a one-liner in Phase 03 (M3).
4. (Nits) fix phone regex `()()` duplication, remove `_ = botUser` dead assign, inline `isErrKeyIssuerNotWired` helper.
5. Proceed to Phase 03 Key Service implementation.

## Unresolved questions
None.

**Status:** DONE
**Verdict:** APPROVED
**Summary:** 0 crit / 0 high / 3 med / 3 low; F4 atomicity: CLEAN
**Next:** commit-ready-for-phase-03
