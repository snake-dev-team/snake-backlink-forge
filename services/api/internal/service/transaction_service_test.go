// Package service_test — integration tests for TransactionService using live Postgres.
// Requires DATABASE_URL env var (from .env or CI). Uses unique users per test.
// Volume + race tests: transaction_service_volume_test.go
package service_test

import (
	"context"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── Harness helpers ───────────────────────────────

// providerRefRegex is the expected format for [F2] order codes.
var providerRefRegex = regexp.MustCompile(`^[A-F0-9]{12}$`)

// insertTestUserLargeRange creates a user with a tgID in a 1B-slot range to avoid
// collisions when creating 1000+ users across test runs.
// Range 5_000_000_000–6_000_000_000 is well outside uniqueTgID's 1M–2M range.
func insertTestUserLargeRange(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	ctx := context.Background()
	for range 10 {
		tgID := int64(5_000_000_000) + rng.Int63n(1_000_000_000)
		var userID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO users (telegram_id) VALUES ($1) RETURNING id`, tgID,
		).Scan(&userID); err != nil {
			continue
		}
		_, _ = pool.Exec(ctx,
			`INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID,
		)
		return userID
	}
	t.Fatal("insertTestUserLargeRange: no unique user after 10 retries")
	return uuid.Nil
}

func newTestTxService(t *testing.T, pool *pgxpool.Pool) *service.TransactionService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	cfg := &config.Config{SepayBankCode: "MBBank", SepayBankAccount: "123456789"}
	return service.NewTransactionService(pool, q, nil, cfg, log.Named("tx_test"))
}

// insertTestUserForTx delegates to the shared insertTestUser helper (key_service_test.go).
func insertTestUserForTx(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	return insertTestUser(t, pool)
}

// countPendingRows returns the count of pending rows for user+package.
func countPendingRows(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, pkgCode string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id=$1 AND package_code=$2 AND status='pending'`,
		userID, pkgCode,
	).Scan(&n); err != nil {
		t.Fatalf("countPendingRows: %v", err)
	}
	return n
}

// getTxRow fetches a full transaction row by primary key for assertion.
func getTxRow(t *testing.T, pool *pgxpool.Pool, txID uuid.UUID) sqlcdb.Transaction {
	t.Helper()
	var tx sqlcdb.Transaction
	if err := pool.QueryRow(context.Background(),
		`SELECT id, user_id, provider, provider_ref, package_code, amount_vnd,
		        premium_granted, standard_granted, status, paid_at, metadata, created_at, updated_at
		 FROM transactions WHERE id=$1`, txID,
	).Scan(
		&tx.ID, &tx.UserID, &tx.Provider, &tx.ProviderRef, &tx.PackageCode,
		&tx.AmountVnd, &tx.PremiumGranted, &tx.StandardGranted, &tx.Status,
		&tx.PaidAt, &tx.Metadata, &tx.CreatedAt, &tx.UpdatedAt,
	); err != nil {
		t.Fatalf("getTxRow %s: %v", txID, err)
	}
	return tx
}

// ─────────────────────────── Tests ─────────────────────────────────────────

func TestCreateTopupIntent_NewRow(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)
	userID := insertTestUserForTx(t, pool)

	tx, qrURL, err := svc.CreateTopupIntent(context.Background(), userID, "standard_pro_200")
	if err != nil {
		t.Fatalf("CreateTopupIntent: %v", err)
	}
	if tx.ProviderRef == nil {
		t.Fatal("ProviderRef is nil")
	}
	if !providerRefRegex.MatchString(*tx.ProviderRef) {
		t.Errorf("provider_ref %q does not match ^[A-F0-9]{12}$", *tx.ProviderRef)
	}
	if tx.AmountVnd != 329_000 {
		t.Errorf("amount_vnd: got %d, want 329000", tx.AmountVnd)
	}
	if !strings.HasPrefix(qrURL, "https://qr.sepay.vn/img") {
		t.Errorf("qrURL prefix wrong: %s", qrURL)
	}
	if !strings.Contains(qrURL, "SBF+TOPUP+"+*tx.ProviderRef) &&
		!strings.Contains(qrURL, "SBF TOPUP "+*tx.ProviderRef) {
		t.Errorf("qrURL missing des with provider_ref: %s", qrURL)
	}
	if n := countPendingRows(t, pool, userID, "standard_pro_200"); n != 1 {
		t.Errorf("pending rows: got %d, want 1", n)
	}
}

func TestCreateTopupIntent_Idempotent(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)
	userID := insertTestUserForTx(t, pool)

	tx1, _, err := svc.CreateTopupIntent(context.Background(), userID, "standard_basic_100")
	if err != nil {
		t.Fatalf("first CreateTopupIntent: %v", err)
	}
	tx2, _, err := svc.CreateTopupIntent(context.Background(), userID, "standard_basic_100")
	if err != nil {
		t.Fatalf("second CreateTopupIntent: %v", err)
	}
	if tx1.ID != tx2.ID {
		t.Errorf("idempotent: different IDs: %s vs %s", tx1.ID, tx2.ID)
	}
	if n := countPendingRows(t, pool, userID, "standard_basic_100"); n != 1 {
		t.Errorf("pending rows after idempotent: got %d, want 1", n)
	}
}

func TestCreateTopupIntent_UnknownPackage(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)
	userID := insertTestUserForTx(t, pool)
	_, _, err := svc.CreateTopupIntent(context.Background(), userID, "nonexistent_package")
	if err != service.ErrUnknownPackage {
		t.Errorf("expected ErrUnknownPackage, got: %v", err)
	}
}

func TestCreateTopupIntent_ConcurrentRace(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)
	userID := insertTestUserForTx(t, pool)

	const goroutines = 10
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, _, errs[i] = svc.CreateTopupIntent(context.Background(), userID, "premium_starter_50")
		}()
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d: %v", i, e)
		}
	}
	if n := countPendingRows(t, pool, userID, "premium_starter_50"); n != 1 {
		t.Errorf("pending rows after race: got %d, want 1", n)
	}
}

func TestCancelPendingTransaction_Q2(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)
	userID := insertTestUserForTx(t, pool)

	createdTx, _, err := svc.CreateTopupIntent(context.Background(), userID, "combo_p100_s50")
	if err != nil {
		t.Fatalf("CreateTopupIntent: %v", err)
	}
	refBefore := createdTx.ProviderRef

	if err := svc.CancelPendingTransaction(context.Background(), userID, createdTx.ID, "user_clicked_cancel"); err != nil {
		t.Fatalf("CancelPendingTransaction: %v", err)
	}

	row := getTxRow(t, pool, createdTx.ID)
	if row.Status != sqlcdb.TransactionStatusCancelled {
		t.Errorf("status: got %q, want cancelled", row.Status)
	}
	if row.ProviderRef == nil || refBefore == nil || *row.ProviderRef != *refBefore {
		t.Errorf("provider_ref changed after cancel")
	}
	if !strings.Contains(string(row.Metadata), "user_clicked_cancel") {
		t.Errorf("metadata missing cancel_reason: %s", row.Metadata)
	}
	if !strings.Contains(string(row.Metadata), "cancelled_at") {
		t.Errorf("metadata missing cancelled_at: %s", row.Metadata)
	}
}

func TestCancelPendingTransaction_RejectsWrongUser(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)
	ownerID := insertTestUserForTx(t, pool)
	otherID := insertTestUserForTx(t, pool)

	createdTx, _, err := svc.CreateTopupIntent(context.Background(), ownerID, "combo_p100_s50")
	if err != nil {
		t.Fatalf("CreateTopupIntent: %v", err)
	}

	if err := svc.CancelPendingTransaction(context.Background(), otherID, createdTx.ID, "wrong_user_cancel"); err != nil {
		t.Fatalf("CancelPendingTransaction: %v", err)
	}

	row := getTxRow(t, pool, createdTx.ID)
	if row.Status != sqlcdb.TransactionStatusPending {
		t.Fatalf("status: got %q, want pending", row.Status)
	}
}

func TestCancelledTxRetainsProviderRef(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)
	userID := insertTestUserForTx(t, pool)

	tx1, _, err := svc.CreateTopupIntent(context.Background(), userID, "standard_max_300")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := svc.CancelPendingTransaction(context.Background(), userID, tx1.ID, "test_cancel"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	tx2, _, err := svc.CreateTopupIntent(context.Background(), userID, "standard_max_300")
	if err != nil {
		t.Fatalf("second create after cancel: %v", err)
	}
	if tx1.ID == tx2.ID {
		t.Error("expected new tx row after cancel, got same ID")
	}
	if tx2.ProviderRef == nil || !providerRefRegex.MatchString(*tx2.ProviderRef) {
		t.Errorf("new provider_ref invalid: %v", tx2.ProviderRef)
	}

	row1 := getTxRow(t, pool, tx1.ID)
	if row1.Status != sqlcdb.TransactionStatusCancelled {
		t.Errorf("cancelled row status: got %q, want cancelled", row1.Status)
	}
}
