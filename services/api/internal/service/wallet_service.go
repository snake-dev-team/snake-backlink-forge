// wallet_service.go — Phase 04: thin wrapper around grant_credits / consume_credits stored procs.
//
// Transaction semantics:
//   - Grant / Consume / AddVNDSpent REQUIRE a pgx.Tx owned by the caller.
//     This enables Phase 06 webhook to compose Grant+Grant+AddVNDSpent in one CAS-gated txn.
//   - GrantStandalone opens its own txn — convenience for admin grants + tests.
//
// Idempotency: NOT this service's responsibility.
//   - Phase 06 webhook gates on: UPDATE transactions SET status='paid' WHERE status='pending' RETURNING ...
//   - Phase 02 trial gate guards via trial_used=false WHERE clause in VerifyContactAndGrantTrial.
//   - total_vnd_spent is NOT incremented by grant_credits proc (see phase spec §Key Insights).
//     AddVNDSpent must be called separately in the same txn by Phase 06 webhook.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// ErrInsufficientCredits is returned by Consume when the user's balance is below the requested amount.
// Translated from Postgres RAISE EXCEPTION 'INSUFFICIENT_CREDITS' USING ERRCODE='P0001'.
var ErrInsufficientCredits = errors.New("insufficient_credits")

// validPools contains the only accepted pool values.
var validPools = map[string]bool{"standard": true, "premium": true}

// GrantInput is the common parameter set for Grant and Consume.
type GrantInput struct {
	UserID    uuid.UUID
	Pool      string // "standard" | "premium"
	Amount    int
	EventType string // must be a valid ledger_event_type enum value
	RefType   string
	RefID     uuid.UUID
}

// WalletService wraps the grant_credits / consume_credits stored procs with
// input validation and typed error translation.
type WalletService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	log  *zap.Logger
}

// NewWalletService constructs a WalletService. q is passed in (not constructed here)
// so callers can share the same *Queries instance across services — same pattern as KeyService.
func NewWalletService(pool *pgxpool.Pool, q *sqlcdb.Queries, log *zap.Logger) *WalletService {
	return &WalletService{pool: pool, q: q, log: log}
}

// Grant calls grant_credits stored proc inside the caller-provided tx.
// Returns the new balance after the grant.
// Validates: amount > 0, pool ∈ {standard, premium}.
//
// PRECONDITION: wallet row MUST exist for in.UserID (ensured by user_service.EnsureStub
// during /start flow). If missing, stored proc's UPDATE ... RETURNING yields 0 rows →
// ledger INSERT with balance_after=NULL violates NOT NULL → raw pgErr 23502 (not
// ErrInsufficientCredits). Callers compose EnsureStub before any Grant/Consume.
func (s *WalletService) Grant(ctx context.Context, tx pgx.Tx, in GrantInput) (int, error) {
	if err := validateInput(in); err != nil {
		return 0, err
	}
	var newBalance int
	err := tx.QueryRow(ctx,
		`SELECT grant_credits($1,$2,$3,$4,$5,$6)`,
		in.UserID, in.Pool, in.Amount, in.EventType, in.RefType, in.RefID,
	).Scan(&newBalance)
	if err != nil {
		return 0, fmt.Errorf("wallet_service.Grant: %w", err)
	}
	return newBalance, nil
}

// Consume calls consume_credits stored proc inside the caller-provided tx.
// Returns the new balance after the consume.
// Translates P0001/INSUFFICIENT_CREDITS → ErrInsufficientCredits.
func (s *WalletService) Consume(ctx context.Context, tx pgx.Tx, in GrantInput) (int, error) {
	if err := validateInput(in); err != nil {
		return 0, err
	}
	var newBalance int
	err := tx.QueryRow(ctx,
		`SELECT consume_credits($1,$2,$3,$4,$5,$6)`,
		in.UserID, in.Pool, in.Amount, in.EventType, in.RefType, in.RefID,
	).Scan(&newBalance)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "P0001" && pgErr.Message == "INSUFFICIENT_CREDITS" {
			return 0, ErrInsufficientCredits
		}
		return 0, fmt.Errorf("wallet_service.Consume: %w", err)
	}
	return newBalance, nil
}

// AddVNDSpent bumps wallets.total_vnd_spent by vnd within the caller-provided tx.
// Called from Phase 06 webhook atomic txn — NOT called by grant_credits proc itself.
func (s *WalletService) AddVNDSpent(ctx context.Context, tx pgx.Tx, userID uuid.UUID, vnd int64) error {
	qtx := sqlcdb.New(tx)
	err := qtx.AddVNDSpent(ctx, sqlcdb.AddVNDSpentParams{
		UserID:        userID,
		TotalVndSpent: vnd,
	})
	if err != nil {
		return fmt.Errorf("wallet_service.AddVNDSpent: %w", err)
	}
	return nil
}

// GetBalance reads the wallet row for userID. No transaction needed (read-only).
// Returns pgx.ErrNoRows wrapped in a descriptive error when no wallet exists.
func (s *WalletService) GetBalance(ctx context.Context, userID uuid.UUID) (sqlcdb.Wallet, error) {
	w, err := s.q.GetWalletByUser(ctx, userID)
	if err != nil {
		return sqlcdb.Wallet{}, fmt.Errorf("wallet_service.GetBalance: %w", err)
	}
	return w, nil
}

// GrantStandalone opens its own ReadCommitted txn, calls Grant, and commits.
// Use only for single-grant flows (admin adjust, tests). NOT for composed flows
// where multiple operations must be atomic — use Grant with a caller-owned tx there.
func (s *WalletService) GrantStandalone(ctx context.Context, in GrantInput) (int, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, fmt.Errorf("wallet_service.GrantStandalone: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	bal, err := s.Grant(ctx, tx, in)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("wallet_service.GrantStandalone: commit: %w", err)
	}
	return bal, nil
}

// validateInput checks amount > 0 and pool is one of the accepted values.
func validateInput(in GrantInput) error {
	if in.Amount <= 0 {
		return fmt.Errorf("amount must be > 0")
	}
	if !validPools[in.Pool] {
		return fmt.Errorf("invalid pool %q: must be \"standard\" or \"premium\"", in.Pool)
	}
	return nil
}
