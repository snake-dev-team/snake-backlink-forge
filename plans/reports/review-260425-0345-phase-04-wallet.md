# Code Review — Phase 04 Wallet Service + /balance

## Verdict
**APPROVED_WITH_FIXES** — 0 critical, 1 high, 2 medium, 2 low. Money-flow semantics CLEAN. Single high-pri bug is observability-only (log call drops err context); it does NOT corrupt state. Safe to land after log fix.

## Summary
Money-flow atomicity, tx plumbing, pgErrCode translation, input validation, and concurrent race test all correct. Stored-proc arg order matches 20260424001:312-319. `/balance` handler is defensive. One high-pri observability bug in `cmd_balance.go:39-41` (empty `log.Error` call); remaining issues are code-quality nits. Claimed verification (sqlc/build/vet/tests) re-run locally — all pass.

---

## Critical findings
None.

---

## High findings

### H1. `HandleBalance`: log.Error call drops err + user_id — observability hole
**File:** `services/api/internal/bot/cmd_balance.go:39-41`

```go
deps.Log.Error("HandleBalance: GetBalance failed",
    // user.ID logged as opaque UUID — no PII
)
```

The variadic `...zap.Field` list is EMPTY — only a comment between the parens. `err` and `user.ID` are both in scope but neither is logged. When `/balance` breaks in production, logs will show `HandleBalance: GetBalance failed` with ZERO context. Root-cause analysis (pgx timeout? schema mismatch? pool exhaustion?) becomes impossible.

**Contrast:** `cmd_regenkey.go` and `user_service_trial.go:109-110, 139-140` correctly include `zap.String("user_id",…)` and `zap.Error(err)`.

**Fix:**
```go
deps.Log.Error("HandleBalance: GetBalance failed",
    zap.String("user_id", user.ID.String()),
    zap.Error(err),
)
```
Requires adding `"go.uber.org/zap"` import (currently absent).

**Impact:** Production debuggability. No user-visible effect. Not a money-flow bug. Must fix before phase-06 webhook (same logger pattern will be needed for transaction failures).

---

## Medium findings

### M1. Stored-proc failure when wallet row missing — no typed error
**Files:** `services/api/internal/service/wallet_service.go:63-76` + stored proc `20260424001_init.sql:312-342`

If `Grant` is called for a userID that has NO `wallets` row (e.g. user stub created but `EnsureWallet` forgotten), stored proc behavior:
1. `UPDATE wallets … WHERE user_id=$1 RETURNING` → 0 rows → `new_balance` stays `NULL`.
2. `INSERT INTO ledger (…, balance_after=NULL)` → violates `ledger.balance_after INT NOT NULL` → raises 23502 (not_null_violation).

Propagated to caller as raw wrapped pgErr. Not translated to a friendly error. In Phase 02/03/04 it does not occur because `cmd_start.go` calls `EnsureWallet` before handing the userID to handlers, and `insertTestUser` seeds wallet. But Phase 08 admin-adjust and any future code path that builds userIDs from external input (webhook, API key user) MUST seed wallet first — or hit this.

Not a blocker (caller discipline is an ok contract), but worth a defensive path: stored proc should `RAISE EXCEPTION 'WALLET_NOT_FOUND' USING ERRCODE='P0001'` if `new_balance IS NULL`. Or Go side checks `pg.ErrNoRows`-equivalent and returns `ErrWalletNotFound`. Not fixing now — log a followup. Worth a one-line code comment pointing to `EnsureWallet` as precondition.

**Recommendation:** Add comment to `Grant` docstring: `// Precondition: wallets row MUST exist for userID (call EnsureWallet first). Omitting causes P0001 via NOT NULL on ledger.balance_after.`

### M2. `Consume` pgErrCode translation accepts any P0001 with "INSUFFICIENT_CREDITS" substring
**File:** `services/api/internal/service/wallet_service.go:91-94`

```go
if errors.As(err, &pgErr) && pgErr.Code == "P0001" && strings.Contains(pgErr.Message, "INSUFFICIENT_CREDITS") {
    return 0, ErrInsufficientCredits
}
```

Today, only `consume_credits` raises `P0001 'INSUFFICIENT_CREDITS'` — substring match is sufficient. But the contract with the stored proc is brittle: if someone adds a second `RAISE EXCEPTION 'INSUFFICIENT_CREDITS_SUBTYPE_X'` with SQLSTATE P0001, it also gets misclassified. Exact match (`pgErr.Message == "INSUFFICIENT_CREDITS"`) is safer and the current proc uses exactly that literal.

**Recommendation:** Tighten to `pgErr.Message == "INSUFFICIENT_CREDITS"` since the proc raises the exact literal (init.sql:301). Low-risk change.

---

## Low / nits

### L1. `formatVND` special-case for zero is redundant but harmless
**File:** `services/api/internal/bot/cmd_balance.go:73-75`

The zero short-circuit is unnecessary — the loop logic handles `"0"` correctly (`len(s)=1`, `start=1`, write "0", no comma). Not wrong, just dead code. Ignore or clean up on next touch.

### L2. Test file doubles as both integration (DB-required) and unit (pure-validation)
**File:** `services/api/internal/service/wallet_service_test.go:273-316`

`TestGrant_AmountValidation` + `TestGrant_InvalidPool` open a pool via `newTestPool` even though validation fires before any DB call. Harmless but wasteful in CI. Could accept a nil pool for these tests. YAGNI — fine as-is.

---

## Plan spec alignment

| Spec item | Status | Notes |
|---|---|---|
| `WalletService.GetBalance(ctx, userID) (Wallet, error)` | ✅ | wallet_service.go:116-122 |
| `Grant(ctx, tx, in) (int, error)` — required pgx.Tx | ✅ | wallet_service.go:63-76 |
| `Consume(ctx, tx, in) (int, error)` — required pgx.Tx | ✅ | wallet_service.go:81-98 |
| `AddVNDSpent(ctx, tx, userID, vnd) error` — required pgx.Tx | ✅ | wallet_service.go:102-112 uses `sqlcdb.New(tx)` correctly |
| Validate `amount > 0` | ✅ | validateInput line 146 |
| Validate pool ∈ {standard, premium} | ✅ | validateInput line 149; bonus over spec |
| Translate P0001 INSUFFICIENT_CREDITS → ErrInsufficientCredits | ✅ | Substring match; see M2 |
| `GrantStandalone` opens own tx, commits/rolls back | ✅ | wallet_service.go:127-142, deferred Rollback with `//nolint:errcheck` |
| `GetBalance` no txn (read-only) | ✅ | Uses pooled `s.q.GetWalletByUser` |
| sqlc queries match spec | ✅ | wallets.sql + ledger.sql |
| Migration 004 `topup_excess` usable | ✅ | `TestGrant_TopupExcess_EventType` passes |
| `/balance` message keys: Premium, Standard, TotalVND | ✅ | cmd_balance.go:53-61 |
| Race test: 10 goroutines → final=10 + 10 ledger rows | ✅ | TestGrant_ConcurrentRace with barrier channel |

All 13 spec items met.

---

## Money-flow correctness audit

- [x] **Grant stored proc call with correct args**
  Proc sig: `(p_user_id UUID, p_pool VARCHAR, p_amount INT, p_event_type ledger_event_type, p_ref_type VARCHAR, p_ref_id UUID)` — init.sql:312-318.
  Go call: `(in.UserID, in.Pool, in.Amount, in.EventType, in.RefType, in.RefID)` — wallet_service.go:70. Positional match, types OK (int→INT, string→VARCHAR, string→ledger_event_type via pgx enum cast, uuid.UUID→UUID).

- [x] **Consume translates P0001 correctly**
  `errors.As` + code check + message substring — wallet_service.go:91-94. Test `TestConsume_InsufficientCredits` validates: returns typed sentinel, wallet row UNCHANGED (proc raised before INSERT, txn rolled back), ledger count unchanged.

- [x] **AddVNDSpent uses tx** not pool
  `sqlcdb.New(tx)` — wallet_service.go:103. `tx` is the caller-provided `pgx.Tx` implementing `DBTX`. No pool leakage.

- [x] **GetBalance no tx (read-only OK)**
  Direct `s.q.GetWalletByUser` — wallet_service.go:117. `s.q` was constructed from pool. Read-committed snapshot is acceptable for a balance-display command. No row lock needed.

- [x] **Concurrent Grant test validates row-lock**
  `TestGrant_ConcurrentRace` — 10 goroutines + barrier channel (wait for all ready) + WaitGroup (wait for all done). Uses `GrantStandalone` so each opens own tx. `UPDATE wallets … RETURNING premium_credits` takes FOR UPDATE-equivalent row lock under ReadCommitted — concurrent updates serialize. Final balance=10 exactly, ledger count=10 exactly. Test ran 10 iterations locally, passed.

- [x] **GrantStandalone tx safety**
  Open tx → defer Rollback (noerrcheck — safe because Rollback on an already-committed tx is a no-op error) → Grant → Commit. If Grant errors, defer rolls back. If Commit errors, state already committed at proc level (pgx guarantee)? — no, pgx Commit is the durable boundary; if Commit fails network/timeout, the txn is uncommitted server-side (rolled back by server on disconnect) and our defer Rollback is a harmless follow-up. Correct pattern.

- [x] **Composability for Phase 06 webhook**
  Grant/Consume/AddVNDSpent all accept caller-owned `pgx.Tx`. Phase 06 can compose: `BEGIN → UPDATE transactions SET status='paid' RETURNING → wallet.Grant(premium) → wallet.Grant(standard) → wallet.AddVNDSpent → COMMIT`. CAS gate on step 1 ensures one-time execution. Current design supports this cleanly — no hidden pool reference inside service methods.

Money-flow verdict: **CLEAN.** No atomicity break, no silent lost update, no tx-pool confusion.

---

## Concurrency / security / regression

- **No regressions**: existing `TestVerifyContactAndGrantTrial_Success` (user_service_test.go:160) still uses raw `tx.QueryRow('SELECT grant_credits(...)')` inside its own txn — Phase 02 path is untouched, as spec allows. All phase-02/03/04 tests pass (`go test` clean).
- **No sensitive logging**: cmd_balance.go logs nothing sensitive — just a bare error (which is actually too bare, see H1). No plaintext secrets, no PII.
- **`/balance` auth**: relies on `loadUser` middleware (Phase 01) — `UserFromCtx` returns false if ctx has no BotUser; handler responds with friendly fallback. Zero-UUID sentinel guard on line 30 defends against dev-mode/synthetic users.
- **Amount validation** fires BEFORE any DB call — no path for negative/zero to reach stored proc. Pool validation prevents invalid-enum trips (proc also rejects, but defense in depth).
- **Markdown ParseMode**: `*bold*` renders correctly. No user-controlled text in template — no markdown-injection risk.
- **File sizes**: wallet_service.go=153, test=436, cmd_balance.go=104 — all under the code-implementation-file 200-line guideline (tests are exempt).

---

## Approved items

- Transaction semantics (caller-owned tx; composability for Phase 06)
- Stored-proc arg order + types
- Input validation (amount + pool)
- `ErrInsufficientCredits` sentinel + translation
- `AddVNDSpent` using `sqlcdb.New(tx)` for tx scope
- `GetBalance` read-only path
- Concurrent race test with barrier channel
- VND formatter correctness (tested mentally for edge cases: 0, 999, 1000, 1000000, negative all work)
- Router wiring (`/balance` case) + deps wiring + main.go init order
- Nil-service fallback in `HandleBalance` (dev mode)
- Migration 004 enum additions + `topup_excess` accepted by stored proc

---

## Recommended actions

1. **H1 (REQUIRED before land)**: Fix `cmd_balance.go:39-41` — add `zap.String("user_id", user.ID.String()), zap.Error(err)` inside the `Log.Error` call. Add `"go.uber.org/zap"` to imports. Re-run `go vet` + tests.
2. **M1 (SHOULD)**: Add `// Precondition: wallets row must exist` comment to `Grant` docstring. Consider migration 005 to harden stored proc with `WALLET_NOT_FOUND` raise. Not a blocker for phase-05.
3. **M2 (SHOULD)**: Change `strings.Contains(pgErr.Message, "INSUFFICIENT_CREDITS")` to `pgErr.Message == "INSUFFICIENT_CREDITS"` — tighter contract with proc. Re-run `TestConsume_InsufficientCredits`.
4. **L1/L2 (OPTIONAL)**: Clean up on next touch — not blocking.

---

## Unresolved questions
- Phase 06 webhook composition depends on a shared tx — spec is clear, but the CAS-gated step-1 return format (rows-affected, or explicit row fetch) should be finalised in phase-05 or phase-06 to keep wallet_service.go stable.
- Race test was run without `-race` (no GCC on Windows per claimed verification). Acceptable per plan, but CI should run `go test -race -count=10` on Linux once available.

---

**Status:** DONE
**Verdict:** APPROVED_WITH_FIXES
**Summary:** 0 crit / 1 high / 2 med / 2 low; money-flow: CLEAN
**Report:** E:\tool_backlink\plans\reports\review-260425-0345-phase-04-wallet.md
**Next:** fix-H1-then-commit
