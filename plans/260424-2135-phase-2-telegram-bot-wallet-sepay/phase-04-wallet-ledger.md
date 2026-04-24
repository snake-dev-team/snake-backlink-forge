# Phase 04 — Wallet Service + Ledger Atomic Ops + `/balance`

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §1.3 credit pools, §3.1 wallets/ledger DDL
- Existing stored procs: `services/api/internal/migrations/20260424001_init.sql` lines 273-342 (`grant_credits`, `consume_credits`)
- Research: `./research/research-01-library-and-concurrency-decisions.md` §3 (pgx isolation)

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** Thin service wrapping existing stored procs with idempotency checks + transaction boundaries. Exposes `/balance` command.

## Key Insights
- **Existing `grant_credits` / `consume_credits` stored procs do NOT enforce idempotency.** Caller MUST wrap in transaction with an idempotency gate (CAS on `transactions.status` or on an explicit `ledger_idempotency` row).
- Wallet service is a THIN WRAPPER — no new stored proc. We keep the procs as-is (they're correct for the atomic math+ledger insert). Idempotency is caller-side.
- `/balance` is read-only; no locking needed.
- `total_vnd_spent` tracked in `wallets` but NOT incremented by `grant_credits` — need separate UPDATE in the same txn in phase 06 (webhook).

## Requirements
### Functional
- `WalletService.GetBalance(ctx, userID) (Wallet, error)`
- `WalletService.Grant(ctx, tx pgx.Tx, userID, pool, amount, eventType, refType, refID) (newBalance int, err error)` — thin wrapper with input validation, calls `grant_credits` stored proc inside provided tx.
- `WalletService.Consume(ctx, tx pgx.Tx, userID, pool, amount, eventType, refType, refID) (newBalance int, err error)` — same pattern.
- `WalletService.AddVNDSpent(ctx, tx pgx.Tx, userID UUID, vnd int64) error` — bumps `total_vnd_spent` in wallets.
- `/balance` command — reply Premium + Standard credits + total VND spent, formatted.

### Non-functional
- Service methods accept `pgx.Tx` (not pool) — caller controls txn. Enables composition in phase 06 webhook.
- Overload: `GrantStandalone(ctx, ...)` helper that opens its own tx (convenience wrapper — DRY).
- Validate `amount > 0` before calling stored proc (defense in depth).
- Translate Postgres errcode `P0001 INSUFFICIENT_CREDITS` into typed Go error `ErrInsufficientCredits`.

## Architecture

```go
// service/wallet_service.go
type WalletService struct {
    pool *pgxpool.Pool
    q    *sqlcdb.Queries
    log  *zap.Logger
}

var ErrInsufficientCredits = errors.New("insufficient_credits")

func (s *WalletService) Grant(ctx context.Context, tx pgx.Tx, in GrantInput) (int, error) {
    if in.Amount <= 0 { return 0, fmt.Errorf("amount must be > 0") }
    var newBalance int
    err := tx.QueryRow(ctx,
        `SELECT grant_credits($1,$2,$3,$4,$5,$6)`,
        in.UserID, in.Pool, in.Amount, in.EventType, in.RefType, in.RefID,
    ).Scan(&newBalance)
    return newBalance, err
}

func (s *WalletService) Consume(...) (int, error) {
    // same pattern; translate pgErrorCode P0001 → ErrInsufficientCredits
}

func (s *WalletService) GetBalance(ctx, userID) (Wallet, error) {
    // direct SELECT via sqlc query, no txn
}
```

### Idempotency pattern for callers (documented here, applied in phase 06)
```
WITHIN TXN:
1. UPDATE transactions SET status='paid' WHERE provider_ref=$1 AND status='pending' RETURNING id, premium_granted, standard_granted, user_id, amount_vnd
   - if RowsAffected == 0 → already processed, return ErrAlreadyProcessed (no error to caller, silent)
2. wallet.Grant(tx, premium...)
3. wallet.Grant(tx, standard...)
4. wallet.AddVNDSpent(tx, amount_vnd)
5. COMMIT
```

The gate is step 1. Steps 2-4 only execute if step 1 returned a pending row.

## Related Code Files
### Create
- `services/api/internal/service/wallet_service.go`
- `services/api/internal/service/wallet_service_test.go`
- `services/api/internal/bot/commands/balance.go`
- `services/api/internal/db/queries/wallets.sql`
- `services/api/internal/db/queries/ledger.sql`

### Modify
- `services/api/internal/db/sqlc/wallets.sql.go` (regenerated)
- `services/api/internal/db/sqlc/ledger.sql.go` (regenerated)
- `services/api/internal/bot/router.go` — route `/balance`

## sqlc queries
```sql
-- services/api/internal/db/queries/wallets.sql

-- name: GetWalletByUser :one
SELECT * FROM wallets WHERE user_id = $1;

-- name: AddVNDSpent :exec
UPDATE wallets SET total_vnd_spent = total_vnd_spent + $2, updated_at = NOW() WHERE user_id = $1;

-- services/api/internal/db/queries/ledger.sql

-- name: GetLedgerPage :many
SELECT * FROM ledger WHERE user_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountLedgerByUser :one
SELECT COUNT(*) FROM ledger WHERE user_id = $1;
```

## Implementation Steps
1. Write `db/queries/wallets.sql` + `ledger.sql`; run `sqlc generate`.
2. Write `service/wallet_service.go` with `Grant`/`Consume`/`AddVNDSpent`/`GetBalance`.
3. Translate pgErrCode P0001 → `ErrInsufficientCredits` using `errors.As` on `*pgconn.PgError`.
4. Write `bot/commands/balance.go`:
   - Call `GetBalance`, render message with tmpl key `balance` + data `{Premium, Standard, TotalVND}`.
5. Write integration test:
   - New user wallet 0/0 → Grant(premium, 100) → balance 100/0, ledger row has `delta=+100, balance_after=100`.
   - Grant(standard, 5, 'trial_grant', 'user', userID) → 100/5.
   - Consume(standard, 3, 'consume_backlink', 'job', jobID) → 100/2.
   - Consume(standard, 999) → ErrInsufficientCredits, ledger untouched.
6. Race test: 10 goroutines Grant(premium, 1) concurrently → final balance = 10, exactly 10 ledger rows.

## Todo List
- [ ] Write `wallets.sql` + `ledger.sql` queries
- [ ] Run `sqlc generate`
- [ ] Implement `WalletService`
- [ ] Implement pgErrCode translation
- [ ] Implement `bot/commands/balance.go`
- [ ] Wire router
- [ ] Unit + integration tests
- [ ] Race test: 10x concurrent Grant
- [ ] Verify ledger row count matches grant count

## Success Criteria
- Grant/Consume math correct under 10x concurrent execution (atomic row lock in `UPDATE wallets ... RETURNING`)
- `ErrInsufficientCredits` surfaced cleanly (not raw PgError)
- `/balance` replies within 500ms
- Ledger rows = grants + consumes (append-only, no UPDATE/DELETE)

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Caller forgets to pass tx → caller manages own inconsistent state | Med | High | Make `Grant/Consume` REQUIRE `pgx.Tx`; `GrantStandalone` is the only variant that opens tx internally |
| Stored proc RAISE EXCEPTION not caught | Low | Med | Unit test: deliberately consume more than balance, assert `ErrInsufficientCredits` |
| `total_vnd_spent` bumped twice on retry | Med | Med | Wrapped inside the CAS-gated txn in phase 06 — only runs once |
| pgx Scan on int8 column for BIGINT fails | Low | Low | Use `int64` for total_vnd_spent, not `int` |

## Security Considerations
- Amount validation: reject `amount <= 0`.
- Audit log fires from caller (phase 02 trial grant, phase 06 webhook, phase 08 admin adjust).
- No plaintext secrets; nothing sensitive in wallet rows.
- `/balance` available only to authenticated users (middleware from phase 01 ensures).

## Next Steps
- Phase 05 uses `wallet.Grant` via shared txn in topup intent path.
- Phase 06 uses `wallet.Grant` + `wallet.AddVNDSpent` in webhook atomic txn.
- Phase 08 admin commands use `wallet.Grant` with `event_type='admin_adjust'`.
