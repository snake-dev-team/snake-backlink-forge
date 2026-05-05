// Package service_test — integration tests for WalletService using live Postgres.
// Requires DATABASE_URL env var (from .env or CI). Each test uses unique user IDs.
// Tables are NOT truncated; unique users prevent inter-test collisions.
//
// Race test (TestGrant_ConcurrentRace) must be run with -race -count=10 in CI.
package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── Harness helpers ───────────────────────────────

func newTestWalletService(t *testing.T, pool *pgxpool.Pool) *service.WalletService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	return service.NewWalletService(pool, q, log.Named("wallet_test"))
}

// ensureWalletUser inserts a user + wallet row; returns userID.
// Re-uses insertTestUser already declared in key_service_test.go (same package).
func ensureWalletUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	return insertTestUser(t, pool)
}

// beginTx opens a ReadCommitted pgx.Tx that auto-rolls back via t.Cleanup.
func beginTx(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatalf("beginTx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

// ledgerCount returns the number of ledger rows for userID.
func ledgerCount(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) int64 {
	t.Helper()
	q := sqlcdb.New(pool)
	n, err := q.CountLedgerByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("ledgerCount: %v", err)
	}
	return n
}

// ─────────────────────────── Tests ─────────────────────────────────────────

// TestGrant_NewWallet: EnsureStub user → wallet 0/0 → GrantStandalone(premium, 100) →
// balance 100 returned, wallet.premium_credits=100, ledger row event_type=topup delta=+100 balance_after=100.
func TestGrant_NewWallet(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()

	userID := ensureWalletUser(t, pool)

	bal, err := svc.GrantStandalone(ctx, service.GrantInput{
		UserID:    userID,
		Pool:      "premium",
		Amount:    100,
		EventType: "topup",
		RefType:   "transaction",
		RefID:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("GrantStandalone: %v", err)
	}
	if bal != 100 {
		t.Errorf("returned balance = %d, want 100", bal)
	}

	// Verify wallet row.
	w, err := svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if w.PremiumCredits != 100 {
		t.Errorf("premium_credits = %d, want 100", w.PremiumCredits)
	}
	if w.StandardCredits != 0 {
		t.Errorf("standard_credits = %d, want 0", w.StandardCredits)
	}

	// Verify ledger row.
	q := sqlcdb.New(pool)
	rows, err := q.GetLedgerPage(ctx, sqlcdb.GetLedgerPageParams{UserID: userID, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("GetLedgerPage: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ledger row count = %d, want 1", len(rows))
	}
	r := rows[0]
	if string(r.EventType) != "topup" {
		t.Errorf("event_type = %q, want topup", r.EventType)
	}
	if r.DeltaCredits != 100 {
		t.Errorf("delta_credits = %d, want 100", r.DeltaCredits)
	}
	if r.BalanceAfter != 100 {
		t.Errorf("balance_after = %d, want 100", r.BalanceAfter)
	}
}

// TestGrant_ThenConsume: Grant(standard,5,trial_grant) + Consume(standard,3,consume_backlink) → balance 2.
func TestGrant_ThenConsume(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()

	userID := ensureWalletUser(t, pool)
	jobID := uuid.New()

	// Grant 5 standard credits.
	_, err := svc.GrantStandalone(ctx, service.GrantInput{
		UserID:    userID,
		Pool:      "standard",
		Amount:    5,
		EventType: "trial_grant",
		RefType:   "user",
		RefID:     userID,
	})
	if err != nil {
		t.Fatalf("GrantStandalone trial_grant: %v", err)
	}

	// Consume 3 standard credits.
	tx := beginTx(t, pool)
	bal, err := svc.Consume(ctx, tx, service.GrantInput{
		UserID:    userID,
		Pool:      "standard",
		Amount:    3,
		EventType: "consume_backlink",
		RefType:   "job",
		RefID:     jobID,
	})
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if bal != 2 {
		t.Errorf("balance after consume = %d, want 2", bal)
	}

	// Verify wallet.
	w, err := svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if w.StandardCredits != 2 {
		t.Errorf("standard_credits = %d, want 2", w.StandardCredits)
	}

	// Verify 2 ledger rows.
	n := ledgerCount(t, pool, userID)
	if n != 2 {
		t.Errorf("ledger row count = %d, want 2", n)
	}
}

// TestConsume_InsufficientCredits: wallet 2 std → Consume(standard,999) → ErrInsufficientCredits,
// wallet unchanged, ledger NOT inserted (proc raised before INSERT — txn rolled back).
func TestConsume_InsufficientCredits(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()

	userID := ensureWalletUser(t, pool)

	// Seed 2 standard credits.
	_, err := svc.GrantStandalone(ctx, service.GrantInput{
		UserID:    userID,
		Pool:      "standard",
		Amount:    2,
		EventType: "trial_grant",
		RefType:   "user",
		RefID:     userID,
	})
	if err != nil {
		t.Fatalf("GrantStandalone seed: %v", err)
	}

	beforeCount := ledgerCount(t, pool, userID)

	// Attempt to consume more than available.
	tx := beginTx(t, pool)
	_, err = svc.Consume(ctx, tx, service.GrantInput{
		UserID:    userID,
		Pool:      "standard",
		Amount:    999,
		EventType: "consume_backlink",
		RefType:   "job",
		RefID:     uuid.New(),
	})
	_ = tx.Rollback(ctx) // explicit rollback (cleanup also rolls back — idempotent)

	if !errors.Is(err, service.ErrInsufficientCredits) {
		t.Fatalf("expected ErrInsufficientCredits, got: %v", err)
	}

	// Wallet unchanged.
	w, err := svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if w.StandardCredits != 2 {
		t.Errorf("standard_credits = %d, want 2 (unchanged)", w.StandardCredits)
	}

	// Ledger NOT written (the stored proc RAISEs before INSERT; rolled back).
	afterCount := ledgerCount(t, pool, userID)
	if afterCount != beforeCount {
		t.Errorf("ledger count changed from %d to %d — expected no new rows", beforeCount, afterCount)
	}
}

// TestGrant_TopupExcess_EventType: Grant(standard,43,'topup_excess') → ledger event_type='topup_excess'.
// Verifies migration 004 enum value is accepted by the stored proc.
func TestGrant_TopupExcess_EventType(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()

	userID := ensureWalletUser(t, pool)
	txID := uuid.New()

	bal, err := svc.GrantStandalone(ctx, service.GrantInput{
		UserID:    userID,
		Pool:      "standard",
		Amount:    43,
		EventType: "topup_excess",
		RefType:   "transaction",
		RefID:     txID,
	})
	if err != nil {
		t.Fatalf("GrantStandalone topup_excess: %v", err)
	}
	if bal != 43 {
		t.Errorf("balance = %d, want 43", bal)
	}

	// Verify ledger event_type.
	q := sqlcdb.New(pool)
	rows, err := q.GetLedgerPage(ctx, sqlcdb.GetLedgerPageParams{UserID: userID, Limit: 5, Offset: 0})
	if err != nil {
		t.Fatalf("GetLedgerPage: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no ledger rows")
	}
	if string(rows[0].EventType) != "topup_excess" {
		t.Errorf("event_type = %q, want topup_excess", rows[0].EventType)
	}
}

// TestGrant_AmountValidation: amount=0 and amount=-5 both return "amount must be > 0".
func TestGrant_AmountValidation(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()
	userID := uuid.New() // no DB row needed — validation fires before any query

	for _, amt := range []int{0, -5} {
		_, err := svc.GrantStandalone(ctx, service.GrantInput{
			UserID:    userID,
			Pool:      "standard",
			Amount:    amt,
			EventType: "topup",
			RefType:   "user",
			RefID:     userID,
		})
		if err == nil {
			t.Errorf("amount=%d: expected error, got nil", amt)
			continue
		}
		if err.Error() != "amount must be > 0" {
			t.Errorf("amount=%d: error = %q, want \"amount must be > 0\"", amt, err.Error())
		}
	}
}

// TestGrant_InvalidPool: pool="invalid" returns an error before hitting the DB.
func TestGrant_InvalidPool(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()
	userID := uuid.New()

	_, err := svc.GrantStandalone(ctx, service.GrantInput{
		UserID:    userID,
		Pool:      "invalid",
		Amount:    10,
		EventType: "topup",
		RefType:   "user",
		RefID:     userID,
	})
	if err == nil {
		t.Fatal("expected error for invalid pool, got nil")
	}
}

// TestAddVNDSpent: total_vnd_spent starts at 0 → +329000 → 329000 → +100000 → 429000.
func TestAddVNDSpent(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()

	userID := ensureWalletUser(t, pool)

	// First bump.
	tx1 := beginTx(t, pool)
	if err := svc.AddVNDSpent(ctx, tx1, userID, 329000); err != nil {
		t.Fatalf("AddVNDSpent 329000: %v", err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}

	w, err := svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance after first bump: %v", err)
	}
	if w.TotalVndSpent != 329000 {
		t.Errorf("total_vnd_spent = %d, want 329000", w.TotalVndSpent)
	}

	// Second bump.
	tx2 := beginTx(t, pool)
	if err := svc.AddVNDSpent(ctx, tx2, userID, 100000); err != nil {
		t.Fatalf("AddVNDSpent 100000: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit tx2: %v", err)
	}

	w, err = svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance after second bump: %v", err)
	}
	if w.TotalVndSpent != 429000 {
		t.Errorf("total_vnd_spent = %d, want 429000", w.TotalVndSpent)
	}
}

// TestGetBalance: read-only fetch returns a fully-populated Wallet struct.
func TestGetBalance(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()

	userID := ensureWalletUser(t, pool)

	w, err := svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if w.UserID != userID {
		t.Errorf("user_id = %v, want %v", w.UserID, userID)
	}
	// Fresh wallet has zero balances.
	if w.PremiumCredits != 0 || w.StandardCredits != 0 {
		t.Errorf("fresh wallet credits not zero: premium=%d standard=%d", w.PremiumCredits, w.StandardCredits)
	}
}

// TestGrant_ConcurrentRace: 10 goroutines call GrantStandalone(premium,1) simultaneously.
// Final balance must equal exactly 10; exactly 10 ledger rows must exist.
// Run with: go test -race -count=10 -run TestGrant_ConcurrentRace ./internal/service/...
func TestGrant_ConcurrentRace(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestWalletService(t, pool)
	ctx := context.Background()

	userID := ensureWalletUser(t, pool)

	const goroutines = 10
	var wg sync.WaitGroup
	barrier := make(chan struct{}) // ensure all goroutines start simultaneously
	errs := make([]error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-barrier // wait for all goroutines to be ready
			_, errs[i] = svc.GrantStandalone(ctx, service.GrantInput{
				UserID:    userID,
				Pool:      "premium",
				Amount:    1,
				EventType: "topup",
				RefType:   "user",
				RefID:     userID,
			})
		}()
	}
	close(barrier) // release all goroutines at once
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}

	// Final balance must be exactly 10.
	w, err := svc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if w.PremiumCredits != int32(goroutines) {
		t.Errorf("premium_credits = %d, want %d", w.PremiumCredits, goroutines)
	}

	// Exactly 10 ledger rows.
	n := ledgerCount(t, pool, userID)
	if n != int64(goroutines) {
		t.Errorf("ledger rows = %d, want %d", n, goroutines)
	}
}
