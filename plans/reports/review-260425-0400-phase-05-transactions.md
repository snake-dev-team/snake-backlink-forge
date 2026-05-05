# Code Review — Phase 05 Transactions + /buy + /topup

## Verdict
APPROVED_WITH_FIXES

## Summary
- 0 critical / 2 high / 2 medium / 3 low.
- Money-flow: **CLEAN on F2, Q2, idempotency, snapshot**. All 5 money-flow audit items verified.
- Critical correctness contracts (12-hex codes, retry, cancel preserves provider_ref, idempotent same-QR) are correctly implemented.
- Two **HIGH** findings: (H1) missing ownership check on `topup:*` callbacks → cross-user tx cancel/status-leak vector if txID is disclosed; (H2) `SEPAY_BANK_ACCOUNT` has no fail-fast guard → empty string builds a broken QR URL that SePay will reject silently at webhook time.
- Verdict is APPROVED_WITH_FIXES because money-flow correctness is sound. The high findings are authz/config-surface not money-math.

## Money-flow correctness audit
- [x] **10 packages exact prices per §1.3** — `packages.go:25-66` verified line-by-line: 99,000 / 179,000 / 329,000⭐ / 459,000 / 499,000 / 899,000 / 1,699,000⭐ / 2,399,000 / 999,000 / 1,799,000. Zero drift. `Featured=true` on `standard_pro_200` + `premium_pro_200` exactly as spec.
- [x] **12-hex order code generation** — `transaction_service.go:79-81`: `u := uuid.New()` INSIDE the loop, `hex.EncodeToString(u[:6])` + `strings.ToUpper`. 48-bit entropy. Uses `crypto/rand` via google/uuid. `math/rand` absent from service code (verified via grep). 1000-sample test (`transaction_service_volume_test.go:14-52`) asserts `^[A-F0-9]{12}$` across all 10 packages.
- [x] **Idempotency gate returns same QR** — `transaction_service.go:106-125`: on `idx_tx_user_pkg_pending` 23505, `GetPendingTxByUserPackage` fetches `existing`, and QR is rebuilt from `existing.AmountVnd` + `existing.ProviderRef` (line 122-123), **not** from the newly-generated `orderCode`. Returned `existing` row has original DB-backed ref. Test `TestCreateTopupIntent_Idempotent:142` asserts same tx.ID.
- [x] **Q2 cancel preserves provider_ref** — `transactions.sql:20-31` sets `status='cancelled'` (not `'failed'`), `WHERE id=$1 AND status='pending'` (idempotent safety). Metadata augmented via `||` (not overwritten). Migration 004 partial index `idx_tx_provider_ref_active` includes `'cancelled'` in its WHERE clause (line 23). Test `TestCancelledTxRetainsProviderRef:217` verifies the cancelled row retains provider_ref AND a new create on same user+pkg succeeds (because user_pkg partial index filters on `status='pending'` only).
- [x] **Snapshot invariant (amount_vnd frozen at insert)** — `transaction_service.go:88-91` inserts `pkg.AmountVND`, `pkg.PremiumCredits`, `pkg.StandardCredits`. The `RETURNING *` clause (`transactions.sql:7`) returns these columns. Phase 06 webhook will read from the row, not the map. Plan §Risk line 228 compliant.

## Critical findings
None.

## High findings

### [H1] Missing ownership check on `topup:check` + `topup:cancel` callbacks
**File:** `services/api/internal/bot/cmd_topup.go:88-107`, `cmd_topup_check.go:23-83`
**Severity:** High (authz bypass vector).
**Problem:** Neither handler verifies that `tx.UserID == ctx.user.ID` (or equivalent) before operating on the row. An attacker who obtains another user's tx UUID (via log leak, shared screen, shoulder-surf, misconfigured observability) can:
  - Fabricate `topup:cancel:<victim_tx_UUID>` → `CancelPendingTransaction` flips *their* pending row to `cancelled`. The DB query scopes only by `WHERE id=$1 AND status='pending'` — no user_id filter. Victim's in-progress payment is cancelled from the bot side (SePay can still recover via Q2 late-payment branch, so financially not destructive — but UX/trust impact is real).
  - Fabricate `topup:check:<victim_tx_UUID>` → `fetchTxByID` returns the victim's row; the handler then sends the attacker a chat message revealing the package name + credit counts ("✅ *Nạp tiền thành công!*\n📦 Gói: ..."). Information disclosure.
**Attack feasibility:** Low-to-medium (UUIDs are 128-bit, not guessable), but defense-in-depth failure. Any future log/debug path that emits tx UUIDs opens this.
**Fix:**
  1. In both handlers, after `uuid.Parse`, load the tx via `fetchTxByID` (or a service method), then compare `tx.UserID` against the caller's `UserFromCtx(ctx).ID`. If mismatch → treat as "not found" and silently ignore (avoid confirm/deny oracle).
  2. Alternatively, add `AND user_id = $2` to the CancelPendingTransaction query and make the service method take `userID` as a second arg.
**Why high, not critical:** money is ultimately safe due to Q2 recovery; the worst-case is UX disruption + info leak. If an attacker had easy tx UUIDs, this would jump to critical.

### [H2] `SEPAY_BANK_ACCOUNT` has no fail-fast guard
**File:** `config/config.go:38`, `service/transaction_service.go:94-97`, `cmd/api/main.go:98-102`
**Severity:** High (silent misconfiguration).
**Problem:** `SEPAY_BANK_ACCOUNT` is an optional env (no `required` tag). On production boot with empty value:
  - `transactionService` initializes successfully.
  - `/buy confirm` → `CreateTopupIntent` succeeds (DB row written, provider_ref generated).
  - `BuildQRURL("MBBank", "", 329000, "SBF TOPUP XXXXXXXXXXXX")` returns `https://qr.sepay.vn/img?acc=&amount=329000&bank=MBBank&des=SBF+TOPUP+XXXXXXXXXXXX&template=compact` — valid URL with `acc=` empty param.
  - SePay returns an error image OR an invalid QR. User scans → bank rejects. Money path silently broken.
  - DB now has orphaned pending rows referencing an unusable QR.
**Fix:** Either:
  1. Mark `SepayBankAccount` as `env:"SEPAY_BANK_ACCOUNT,required"` in config.go — fail boot in production. Dev can still run with a placeholder.
  2. Or in `NewTransactionService`, return `(nil, error)` if `cfg.SepayBankAccount == ""` AND `cfg.Env == "production"`, and in dev log.Warn then skip wiring TxService (bot falls back to "coming soon").
  3. Or in `CreateTopupIntent` fail early with `ErrBankAccountMissing` when `s.cfg.SepayBankAccount == ""` — keeps pending rows from being written.
**Recommended:** Option 1 (lift `required` tag) — simplest + matches pattern of DATABASE_URL/REDIS_URL. Local dev already requires a placeholder in .env.example anyway.

## Medium findings

### [M1] sqlc-generated `Column2` parameter name leaks through service layer
**File:** `services/api/internal/db/sqlc/transactions.sql.go:26-27`, `service/transaction_service.go:152`
**Problem:** `CancelPendingTransactionParams.Column2` — sqlc didn't infer a sensible name because the SQL uses `$2::text` without a named CTE or typed parameter annotation. Service code passes `Column2: reason` — unreadable. Future maintainers won't know what `Column2` is.
**Fix:** In `transactions.sql:28-29`, name the param via sqlc syntax:
```sql
metadata = metadata || jsonb_build_object(
    'cancel_reason', sqlc.arg(reason)::text,
    'cancelled_at',  to_char(NOW(), 'YYYY-MM-DD"T"HH24:MI:SSOF')
)
```
Regenerate sqlc → field becomes `Reason`. Service code becomes `Reason: reason`.
**Severity:** Medium (maintainability / readability; no runtime impact).

### [M2] `isNoRows` sentinel check is fragile
**File:** `services/api/internal/service/transaction_service.go:194-197`
**Problem:** Uses `err.Error() == "no rows in result set"` string-match to detect pgx no-rows. Any pgx message string change (or wrapping) breaks the check.
**Fix:** Use `errors.Is(err, pgx.ErrNoRows)` with import `github.com/jackc/pgx/v5`. Type-safe and future-proof.
**Severity:** Medium.

## Low / nits

### [L1] `u` UUID generated at line 79 is not used as the tx primary key
**File:** `transaction_service.go:79-91`
**Note:** Plan spec §CreateTopupIntent (phase-05.md line 85) inserts `id=$1` explicitly. Implementation omits id from insert and lets Postgres DEFAULT generate it. Result: the `u` UUID's bytes[6:] are discarded; only bytes[:6] matter. Not a bug — cleaner in fact — but worth noting. Harmless.

### [L2] `transaction_service_test.go` is 245 lines (>200 cap)
**File:** `services/api/internal/service/transaction_service_test.go`
**Note:** Developer acknowledged this in the plan; volume test was split off to `transaction_service_volume_test.go`. The remaining 245 lines are cohesive test harness + 5 integration tests. Acceptable for test files — further split would scatter the shared helpers (`newTestTxService`, `getTxRow`, `countPendingRows`). Low priority.

### [L3] `handleTopupCancelCallback` answers the callback *before* doing work
**File:** `cmd_topup.go:92`
**Note:** `api.Request(NewCallback(..., ""))` at line 92 fires before the cancel DB operation at line 102. This is fine for Telegram (it only cares about 30s timeout), and it means even if cancel fails, the spinner stops. But if you wanted the alert text to show an error state on cancel failure, you'd need to flip the order or use two callback-ack calls. Current UX is: spinner vanishes immediately, cancel may or may not succeed (caption edit is the only signal). Acceptable for MVP.

## Plan spec alignment

| Requirement | Status | Note |
|---|---|---|
| `/buy` → inline keyboard 3 rows | Pass | `keyboards_buy.go:40-59` — 4+4+2 layout |
| Callback `buy:pkg:<code>` → summary + confirm/cancel | Pass | `cmd_buy.go:75-117` |
| Confirm → CreateTopupIntent → QR photo + caption | Pass | `cmd_buy_confirm.go:24-102` |
| FSM `topup_waiting` TTL 24h | Pass | `cmd_buy.go:28` `topupWaitingTTL = 24*time.Hour`; saved at `cmd_buy_confirm.go:99` |
| `/topup` alias: idle → /buy; waiting → resend QR | Pass | `cmd_topup.go:25-40` |
| `topup:check` polls DB, success/waiting replies | Pass | `cmd_topup_check.go:23-83` — distinguishes `paid` / `recovered_by_late_payment` / still pending |
| `topup:cancel` flips `cancelled` (not `failed`) | Pass | `transactions.sql:24` |
| Idempotent CreateTopupIntent returns same QR | Pass | `transaction_service.go:106-125` — QR rebuilt from DB row |
| 12-hex uppercase provider_ref | Pass | `transaction_service.go:80-81` |
| Retry 3x on provider_ref collision | Pass | `transaction_service.go:78` loop, line 127-133 on constraint-name match |
| CancelPending Q2 semantics | Pass | status='cancelled', metadata augmented, `WHERE status='pending'` safety gate |
| Race: 10x concurrent → 1 row | Pass | `TestCreateTopupIntent_ConcurrentRace:160` |
| 1000-sample provider_ref format | Pass | `transaction_service_volume_test.go:14` |
| Cancelled tx retains provider_ref | Pass | `TestCancelledTxRetainsProviderRef:217` + partial index `status IN ('pending','paid','cancelled','recovered_by_late_payment')` |
| Package code regex guard | Pass | `packages.go:70-84` `ValidatePackageCode` used in both buy and confirm handlers |
| SQLi via callback | Pass | All package_code uses go through `ValidatePackageCode` (map lookup + regex) before DB; all txID uses go through `uuid.Parse` before DB. No string interpolation. |
| Bank creds not logged | Pass | No `zap.*BankAccount` matches in codebase |

## Test adequacy
- Service-layer tests (8) thoroughly cover F2 + Q2 contracts, idempotency, concurrency, unknown package, and 1000-sample format check.
- QR unit tests (4) cover URL prefix, param presence, space-encoding, large-int decimal format.
- **Missing but acceptable for skeleton:**
  - Bot handler unit tests (would require tgbotapi mocking — out of scope).
  - **Authz test** for topup:cancel / topup:check covering cross-user tx-UUID attack (see H1). Add after fix.
  - Test for `BuildQRURL("", "", 0, "")` empty-input behaviour (see H2) — useful to lock in whatever fail-closed behaviour is chosen.
  - No test for the `idx_tx_provider_ref_active` retry branch — the 48-bit-collision path is untested. Hard to trigger in integration; a service-level test that pre-inserts a clashing ref under a faked UUID source would give confidence. Low priority.
- Race with `-race` flag was skipped on Windows (no GCC). Consider running in CI on Linux.

## Approved items
- Package registry integrity (10 bundles, exact prices, correct Featured flags).
- F2 order-code generator (crypto-random source, 48-bit entropy, retry loop with fresh UUID each attempt).
- Q2 cancel semantics (status='cancelled', metadata augmented, WHERE-status='pending' gate, provider_ref preserved in active-state partial index).
- Idempotent CreateTopupIntent (QR rebuilt from DB row not new orderCode — subtle and correct).
- Snapshot invariant (amount/premium/standard frozen at INSERT).
- QR URL builder (pure function, deterministic param ordering via url.Values.Encode, space→`+` consistent with SePay docs).
- Callback security against SQLi (package code via map+regex, txID via uuid.Parse).
- Router wiring (5 new callback prefixes, all ack'd, Message nil-guard via guardCallback).
- No secret-leaks in logs (bank account never passed to zap).
- Config + env discipline (.env.example documents both new vars; SEPAY_BANK_CODE defaults to MBBank per Q3).

## Recommended actions

**Before landing:**
1. **[H1]** Add ownership check in `handleTopupCancelCallback` and `handleTopupCheckCallback`: load tx, compare `tx.UserID` to `UserFromCtx(ctx).ID`, silently ignore mismatch. Add a regression test.
2. **[H2]** Make `SEPAY_BANK_ACCOUNT` required OR fail CreateTopupIntent when empty. Boot-time failure preferred (simpler).

**Follow-up (non-blocking):**
3. **[M1]** Name the `CancelPendingTransaction` parameter via `sqlc.arg(reason)::text` so the field becomes `Reason` instead of `Column2`. Regenerate.
4. **[M2]** Replace `err.Error() == "no rows in result set"` with `errors.Is(err, pgx.ErrNoRows)` in `isNoRows`.
5. Consider running integration tests with `-race` in Linux CI to catch concurrency regressions the Windows dev loop misses.

## Unresolved questions
- Is cross-user topup:cancel considered an acceptable risk by product? (Q2 recovery means financial loss is zero, but the cancel UX disruption + info leak via topup:check are distinct issues.) If no, H1 is blocking; if yes, downgrade H1 to Medium and document the assumption.
- For H2, should we rely on boot-fail (required env) or runtime-fail (empty-guard in service)? Boot-fail is cleaner but means any dev environment missing the var can't run /buy. A required field with placeholder in `.env.example` looks best.
