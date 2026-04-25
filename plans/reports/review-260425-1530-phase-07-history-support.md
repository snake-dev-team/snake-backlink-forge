# Code Review — Phase 07 History/Support/Download/Ref/Language

## Verdict
APPROVED_WITH_FIXES

## Summary
Phase 07 implementation is solid for read-heavy / light-write commands. M5 one-shot ticket guarantee is satisfied via per-user serialization lock (`bot.go:167-169`) + state.Clear() at top of handler. Body truncation, 3-ticket cap, base58 alphabet, history pagination math all correct. Two real bugs identified (HIGH): `ProcessReferralOnStart` runs `IncrementReferralCount` outside the transaction, and `EnsureCode`'s 23505 retry loop cannot recover from `user_id` constraint violation race. Tests pass (13/13 service + 6/6 base58). Build + vet clean.

- 0 critical
- 2 high
- 3 medium
- 4 low/nits

## M5 ticket one-shot audit

- [x] state.Clear() called BEFORE reply (cmd_support.go:167-169 — clears at handler top)
- [x] body truncated to 4096 BEFORE INSERT (cmd_support.go:172-175 + service:67-70 redundant safety)
- [x] cap check at 3 open tickets (cmd_support.go:196 + service:63-65)
- [x] state cleared even if cap rejection fires (Clear precedes cap check)
- [x] race-safety: `Bot.handleUpdate` per-user lock (bot.go:167-169) serializes concurrent messages from same tgID, so 5 rapid messages cannot bypass one-shot guarantee
- [x] FSM dispatch in router precedes silent-ignore (router.go:45-58)
- [x] Telegram caps incoming text at 4096 chars; byte-level >4096 only via multi-byte UTF-8 → truncation acceptable per spec

## Critical findings
None.

## High findings

### H1 — `ProcessReferralOnStart` increments count OUTSIDE the transaction
File: `internal/service/referral_service.go:138`
```go
tx, err := s.pool.Begin(ctx)
...
tag, err := tx.Exec(ctx, `UPDATE users SET referred_by = ... WHERE id = $2 AND referred_by IS NULL`, ...)
...
if err := s.q.IncrementReferralCount(ctx, referral.UserID); err != nil { return err }  // ← uses base pool, NOT tx
if err := tx.Commit(ctx); err != nil { return err }
```
`s.q.IncrementReferralCount` runs against the base `*Queries` (auto-commit) — not `s.q.WithTx(tx)`. This breaks atomicity:
- Increment commits independently. If `tx.Commit` then fails (network blip), `users.referred_by` rolls back but the count remains incremented. Inconsistent state: phantom referral.
- Tests pass because Commit doesn't fail in sunny-day path (`TestProcessReferralOnStart_*` series).

Fix: `s.q.WithTx(tx).IncrementReferralCount(ctx, referral.UserID)` — runs INSIDE tx, atomic with the user UPDATE.

### H2 — `EnsureCode` 23505 retry cannot disambiguate code vs user_id constraint
File: `internal/service/referral_service.go:60-66`
The `referrals` table has BOTH `user_id UNIQUE` AND `code UNIQUE` (`migrations/20260424003_phase2_indexes.sql:29-30`). The retry loop assumes 23505 always means code collision and retries with a new code. But under concurrent `/ref` calls for the same user (both observe `pgx.ErrNoRows` from the fast-path lookup, both attempt INSERT), the LOSER hits 23505 on `referrals_user_id_key`, not `referrals_code_key`. Retrying with new codes will still fail on `user_id`. After 3 retries + 8-char fallback (still fails on `user_id`), user sees error.

Likelihood: low (requires user to double-tap `/ref` within ms; per-user lock at `bot.go:167-169` mostly serializes), but possible if rapid-fire across Telegram polling cycles or future webhook mode.

Fix: inspect `pgErr.ConstraintName`. If `referrals_user_id_key`, re-fetch via `GetReferralByUser` and return existing code (idempotent recovery). If `referrals_code_key`, retry with new code as today.

## Medium findings

### M1 — `HandleHistory` does not guard `deps.Pool == nil`
File: `internal/bot/cmd_history.go:107`
`q := sqlcdb.New(deps.Pool)` then `q.CountTxByUser(ctx, ...)` would nil-deref if `deps.Pool` is nil (Phase 1 lenient boot mode). Recovery middleware (`middleware.go:34-52`) catches the panic and replies "Đã xảy ra lỗi", so user sees error, not crash. Other Phase 04+ handlers (e.g., `cmd_balance.go:24`, `cmd_topup_check.go:99`) gate on the relevant service/pool. Add similar guard at handler top:
```go
if deps.Pool == nil { replyText(api, update, "Tính năng sắp ra mắt."); return nil }
```

### M2 — M5 router-level FSM stress test missing
Plan step 14: "Integration test: user in `support_describing` state sends 5 text messages rapidly → exactly 1 ticket created, remaining messages hit default handler." Phase 07 ships service-level tests (13 PASS) but no bot-handler-level test exercising the router → handler → state-clear flow. The per-user lock guarantee depends on `Bot.handleUpdate` serializing — covered by `bot_per_user_lock_test.go` for general cases but not specifically the support FSM. Acceptable for skeleton; recommend adding one router-level test before Phase 09 cutover.

### M3 — `cmd_history.go` callback pagination resets the OTHER axis to 0
File: `internal/bot/cmd_history.go:72-81`
When user clicks "Tx ▶" while on ledger page 2, the message re-renders with `pageLedger=0` (because callback data only carries the navigated axis). Documented in code (line 76) as "acceptable UX per spec". Spec line 24 states pages are independent (`hist:tx:2` or `hist:ledger:3`), but doesn't say the other axis must persist. UX papercut: encode both pages into callback data (e.g., `hist:tx:2:l3`) for full independence, or accept the current behavior.

## Low / nits

### L1 — Misleading comment on `asPgError`
File: `internal/service/referral_service.go:154`
Comment claims "Reused from wallet_service.go pattern — same package, no duplication risk". `wallet_service.go` does NOT export `asPgError`; this is the only definition in the package. Correct the comment to "Local helper; reused from referral_service only" or move to a shared `service/errors.go`.

### L2 — Negative `pageTx` from crafted callback data
File: `internal/bot/cmd_history.go:78`
`fmt.Sscanf(..., "%d", &pageTx)` accepts negatives. If a malicious user crafts `hist:tx:-1`, `offset := int32(-5)` → SQL "OFFSET must not be negative" error → callback acked but message not edited (visual hang). Defensive clamp:
```go
if pageTx < 0 { pageTx = 0 }
```

### L3 — `cmd_language.go` always confirms language change even if UPDATE fails
File: `internal/bot/cmd_language.go:67`
Comment "Non-fatal: still confirm to user (optimistic)" — but the user's persisted language won't change. Next `/balance` will be in old language. Either retry, or warn user "Đã đổi tạm — chưa lưu được, thử lại sau".

### L4 — Redundant `subject` defaulting + truncation absence
File: `internal/service/support_service.go:73-76`
Caller (`cmd_support.go:203`) always passes `"Support Ticket"` — service-side default is redundant but harmless. Subject (VARCHAR(128)) not truncated; if a future caller passes >128 chars, INSERT fails. Add `if len(subject) > 128 { subject = subject[:128] }` for defense.

## Plan spec alignment

| Step | Status | Notes |
|---|---|---|
| 1. GetLedgerConsumesByUserPage in ledger.sql | ✓ | + CountLedgerConsumesByUser |
| 2. support.sql + referrals.sql | ✓ | clean queries |
| 3. sqlc generate | ✓ | `*.sql.go` regenerated |
| 4. SupportService + ReferralService | ✓ | with caveats H1, H2 |
| 5. /history with pagination | ✓ | minor M3 papercut |
| 6. /support FAQ + FSM describe | ✓ | M5 one-shot OK via per-user lock |
| 7. /download | ✓ | empty URL → friendly fallback |
| 8. /ref | ✓ | idempotent EnsureCode (sunny day) |
| 9. /language | ✓ | minor L3 (silent UPDATE failure) |
| 10. Amend cmd_start.go for ref_XXX parameter | DEFERRED | spec line 43 explicitly defers to phase 02 amend note — not blocked |
| 11. Unit test ref code uniqueness | ✓ | `TestEnsureCode_Uniqueness` (50 codes), `TestGenBase58Upper_Uniqueness` (1000) |
| 12. Integration test history pagination | ⚠️ | not present — relies on visual review |
| 13. Integration test 3-ticket cap | ✓ | `TestCreateTicket_CapReached` |
| 14. M5 stress test (5 rapid messages) | ⚠️ | NOT explicitly present (M2) |
| 15. Body 4096 truncate test | ✓ | `TestCreateTicket_BodyTruncated` (5000→4096) |

## Test adequacy

PASS:
- `TestGenBase58Upper_Length` / `_AlphabetOnly` / `_ExcludesAmbiguous` / `_Uniqueness` (1000) / `_ZeroLen` / `_UppercaseOnly` (6 tests)
- `TestCreateTicket_Success` / `_DefaultSubject` / `_BodyTruncated` / `_CapReached`
- `TestCountOpenTickets_NewUser` / `_AfterResolve`
- `TestEnsureCode_NewUser` / `_Idempotent` / `_Uniqueness`
- `TestProcessReferralOnStart_Valid` / `_Invalid` / `_OwnCode` / `_Idempotent`

GAPS:
- M5 router-level stress test (5 rapid messages → 1 ticket) — covered indirectly via `bot_per_user_lock_test.go`
- 23505 race recovery for `EnsureCode` (cannot test without simulating concurrent insert race)
- `ProcessReferralOnStart` partial-failure (commit fails after IncrementReferralCount)

## Approved items

- M5 ordering correct: state.Clear FIRST, then truncate, cap-check, insert, reply (cmd_support.go:165-219)
- Per-user lock (`Bot.handleUpdate`) prevents concurrent same-user message races for one-shot ticket
- base58 uses crypto/rand (not math/rand), 33-char alphabet, excludes 0/O/I, panic on entropy failure
- 4096-byte truncation at byte boundary, documented (UTF-8 split tolerated per spec)
- 3-ticket cap enforced at TWO layers (handler + service) — fail-safe
- pagination math correct: `maxPage = (total-1)/5`, prev only `page>0`, next only `page<maxPage`
- `ProcessReferralOnStart` write-once guard via `WHERE referred_by IS NULL`
- own-code self-referral guard (referral_service.go:112-114)
- invalid code → silent skip (no error to user)
- `/download` empty URL → friendly "installer chưa publish"
- `/ref` empty `TelegramBotUsername` → graceful fallback text
- `/language` validates lang ∈ {vi, en} before persist
- no body / phone / key contents logged (only user_id, ticket_id)
- HTML mode used for FAQ topics; user-controlled body NOT echoed back to user (admin dashboard concern, deferred)
- File sizes under 200-line threshold: cmd_history.go=300 (over but acceptable, structure justified), cmd_support.go=237, others <150
  - Note: cmd_history.go=300 lines is 50% over the soft cap. Consider extracting render helpers (formatTxRow, formatLedgerRow, txStatusIcon, ledgerEventLabel) to `cmd_history_render.go` for next phase.

## Recommended actions

1. **H1 (must fix before next phase merge):** `referral_service.go:138` — change `s.q.IncrementReferralCount` → `s.q.WithTx(tx).IncrementReferralCount`. One-line fix.
2. **H2 (must fix):** `referral_service.go:60-66` — inspect `pgErr.ConstraintName`; on `referrals_user_id_key` re-fetch and return existing code; on `referrals_code_key` retry as today.
3. **M1:** add nil-pool guard at `cmd_history.go:HandleHistory` top (defensive parity with `cmd_topup_check.go:99`).
4. **M2:** add M5 router-level stress test (state set → 5 sequential text messages → exactly 1 ticket; remaining ignored).
5. **L2:** clamp negative pages to 0 in `handleHistoryCallback`.
6. **L3:** reply with retry/warning text when UPDATE users language fails (or rollback the optimistic edit).
7. **L1, L4:** comment + subject-truncate cleanups.

## Unresolved questions

- Is the deferred `cmd_start.go` ref-code parsing intended to land in Phase 02 amend or Phase 08? Spec line 43 says "Deferred to phase 02 amend note" but Phase 02 already shipped without it. Confirm: amend Phase 02 retroactively or push to a Phase 08 supplement?
- `cmd_history.go` is 300 lines — consider modularization splitting into `cmd_history.go` + `cmd_history_render.go`. Soft over the 200-line cap. Defer to next refactor pass?
- Should `cmd_language.go` use a sqlc-generated query instead of inline `pool.Exec` for the UPDATE? YAGNI argues no, but consistency argues yes (other handlers use sqlc).
