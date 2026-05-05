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
- **[F2 + Q1 + Q2] Schema delta migration** `20260424004_phase2_schema_deltas.sql` required to unblock phase-05/06: adds `topup_excess` ledger_event_type, `recovered_by_late_payment` transaction_status, drops unconditional UNIQUE on `provider_ref`, adds partial UNIQUE on active states only. Ordered AFTER `20260424003_phase2_indexes.sql`.

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
- **[F2+Q1+Q2] `services/api/internal/migrations/20260424004_phase2_schema_deltas.sql`** — ENUM additions + provider_ref partial UNIQUE (see SQL below)

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

## Migration `20260424004_phase2_schema_deltas.sql` — [F2 + Q1 + Q2]

Adds two enum values + replaces base `provider_ref` UNIQUE constraint with a partial UNIQUE scoped to active states.

```sql
-- +goose Up
-- +goose NO TRANSACTION
-- Reason for NO TRANSACTION: ALTER TYPE ADD VALUE cannot run inside a txn block in Postgres.

-- Q1: new ledger event for bonus credits on over-payment
ALTER TYPE ledger_event_type ADD VALUE IF NOT EXISTS 'topup_excess';

-- Q2: new transaction statuses for late-payment recovery
ALTER TYPE transaction_status ADD VALUE IF NOT EXISTS 'cancelled';
ALTER TYPE transaction_status ADD VALUE IF NOT EXISTS 'recovered_by_late_payment';

-- F2: drop unconditional UNIQUE on provider_ref (constraint name from 20260424001)
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_provider_ref_key;

-- F2: partial UNIQUE limited to active states only; failed/refunded rows can share provider_ref with new attempts.
-- 'cancelled' is kept in the active set so a late webhook can transition to 'recovered_by_late_payment'.
CREATE UNIQUE INDEX IF NOT EXISTS idx_tx_provider_ref_active
  ON transactions(provider_ref)
  WHERE status IN ('pending','paid','cancelled','recovered_by_late_payment');

-- +goose Down
-- FORWARD-ONLY NOTE: This migration is effectively forward-only in this deployment due to partial index rebuild + enum value adds.
-- Rollback requires manual DDL (destructive — not automated here):
--   1. DROP INDEX idx_tx_provider_ref_active → restore base UNIQUE constraint.
--   2. Remove enum values (destructive): CREATE TYPE replacement → ALTER COLUMN USING cast → DROP old TYPE → rename replacement.
-- Postgres CAN technically remove enum values via type recreate + data migration, but this is invasive and unnecessary in practice
-- (added values remain unused post-revert without harm). Automated down here only reverts the provider_ref constraint.

DROP INDEX IF EXISTS idx_tx_provider_ref_active;
-- Restore unconditional UNIQUE (may fail if duplicate provider_ref rows exist; accept manual cleanup)
ALTER TABLE transactions ADD CONSTRAINT transactions_provider_ref_key UNIQUE (provider_ref);
```

**Deploy ordering:** `20260424001_init.sql` → `20260424002_seed_dorks.sql` → `20260424003_phase2_indexes.sql` → `20260424004_phase2_schema_deltas.sql`. Goose applies in lexicographic order; file names enforce sequence.

## Implementation Steps
1. **[F2+Q1+Q2]** Write `migrations/20260424004_phase2_schema_deltas.sql` with goose Up/Down sections.
2. Apply migration locally (`make migrate-up`) and verify:
   - `SELECT unnest(enum_range(NULL::ledger_event_type))` contains `topup_excess`
   - `SELECT unnest(enum_range(NULL::transaction_status))` contains `recovered_by_late_payment`
   - `\d transactions` shows `idx_tx_provider_ref_active` partial UNIQUE, no base UNIQUE on `provider_ref`
3. Write `db/queries/wallets.sql` + `ledger.sql`; run `sqlc generate`.
4. Write `service/wallet_service.go` with `Grant`/`Consume`/`AddVNDSpent`/`GetBalance`.
5. Translate pgErrCode P0001 → `ErrInsufficientCredits` using `errors.As` on `*pgconn.PgError`.
6. Write `bot/commands/balance.go`:
   - Call `GetBalance`, render message with tmpl key `balance` + data `{Premium, Standard, TotalVND}`.
7. Write integration test:
   - New user wallet 0/0 → Grant(premium, 100) → balance 100/0, ledger row has `delta=+100, balance_after=100`.
   - Grant(standard, 5, 'trial_grant', 'user', userID) → 100/5.
   - Consume(standard, 3, 'consume_backlink', 'job', jobID) → 100/2.
   - Consume(standard, 999) → ErrInsufficientCredits, ledger untouched.
   - **[Q1]** Grant(standard, 43, 'topup_excess', 'transaction', txID) → ledger row with event_type='topup_excess'.
8. Race test: 10 goroutines Grant(premium, 1) concurrently → final balance = 10, exactly 10 ledger rows.

## Todo List
- [ ] **[F2+Q1+Q2]** Write `migrations/20260424004_phase2_schema_deltas.sql` (ENUM additions + provider_ref partial UNIQUE)
- [ ] Apply migration, verify ENUM values + partial index present
- [ ] Write `wallets.sql` + `ledger.sql` queries
- [ ] Run `sqlc generate`
- [ ] Implement `WalletService`
- [ ] Implement pgErrCode translation
- [ ] Implement `bot/commands/balance.go`
- [ ] Wire router
- [ ] Unit + integration tests
- [ ] Race test: 10x concurrent Grant
- [ ] **[Q1]** Ledger test: event_type='topup_excess' renders correctly in /history
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
