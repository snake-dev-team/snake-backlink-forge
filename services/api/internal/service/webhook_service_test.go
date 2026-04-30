// webhook_service_test.go — integration tests for WebhookService.ProcessPaidTransaction.
// Requires DATABASE_URL env var. Uses live Postgres; tables not truncated (unique users).
//
// Covered invariants:
//
//	[F1] 3-way amount branch: underpaid / exact / overpaid + bonus + admin alert
//	[Q2] CAS widened to ('pending','cancelled') — late payment recovery
//	[F2] Case-insensitive order code normalization
//	Idempotency: second call → AlreadyProcessed, wallet unchanged
//	Concurrency: 10 goroutines same orderCode → exactly 1 grant (CAS gate)
//	UnknownOrder, MixedCaseMemo
package service_test

import (
	"context"
	"encoding/hex"
	"math/rand"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── Harness helpers ───────────────────────────────

// randOrderCode generates a random 12-char uppercase hex order code for test isolation.
// DB is not truncated between runs; unique codes prevent idx_tx_provider_ref_active collisions.
func randOrderCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b) //nolint:gosec // test-only randomness
	return strings.ToUpper(hex.EncodeToString(b))
}

func newTestWebhookService(t *testing.T, pool *pgxpool.Pool, alertCh chan<- notify.AdminAlert) *service.WebhookService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	walletSvc := service.NewWalletService(pool, q, log.Named("wallet"))
	cfg := &config.Config{SepayBankCode: "MBBank", SepayBankAccount: "123456789"}
	return service.NewWebhookService(pool, walletSvc, cfg, log.Named("webhook"), alertCh)
}

// insertPendingTx inserts a pending transaction and returns its ID + order code.
// pkgCode must exist in service.Packages.
func insertPendingTx(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, orderCode string, pkgCode string) uuid.UUID {
	t.Helper()
	pkg := service.Packages[pkgCode]
	var txID uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO transactions
			(user_id, provider, provider_ref, package_code, amount_vnd, premium_granted, standard_granted, status)
		VALUES ($1, 'sepay', $2, $3, $4, $5, $6, 'pending')
		RETURNING id`,
		userID, orderCode, pkgCode, pkg.AmountVND,
		int32(pkg.PremiumCredits), int32(pkg.StandardCredits),
	).Scan(&txID)
	if err != nil {
		t.Fatalf("insertPendingTx: %v", err)
	}
	return txID
}

// getWalletBalance returns (standard_credits, premium_credits) for userID.
func getWalletBalance(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) (standard, premium int32) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT standard_credits, premium_credits FROM wallets WHERE user_id=$1`, userID,
	).Scan(&standard, &premium)
	if err != nil {
		t.Fatalf("getWalletBalance: %v", err)
	}
	return
}

// getTxStatus returns the status string for a transaction by ID.
func getTxStatus(t *testing.T, pool *pgxpool.Pool, txID uuid.UUID) string {
	t.Helper()
	var st string
	err := pool.QueryRow(context.Background(),
		`SELECT status::text FROM transactions WHERE id=$1`, txID,
	).Scan(&st)
	if err != nil {
		t.Fatalf("getTxStatus: %v", err)
	}
	return st
}

// countLedgerByEventType counts ledger rows for userID with a specific event_type.
func countLedgerByEventType(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, eventType string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM ledger WHERE user_id=$1 AND event_type=$2::ledger_event_type`, userID, eventType,
	).Scan(&n)
	if err != nil {
		t.Fatalf("countLedgerByEventType: %v", err)
	}
	return n
}

// buildPayload constructs a minimal SePay Payload for a given order code and amount.
func buildPayload(orderCode string, transferAmount int64) sepay.Payload {
	return sepay.Payload{
		ID:             1,
		Gateway:        "MBBank",
		AccountNumber:  "123456789",
		Content:        "SBF TOPUP " + orderCode,
		TransferType:   "in",
		TransferAmount: transferAmount,
	}
}

// ─────────────────────────── Tests ─────────────────────────────────────────

func TestProcessPaidTransaction_Happy(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	txID := insertPendingTx(t, pool, userID, orderCode, "standard_pro_200")

	svc := newTestWebhookService(t, pool, nil)
	result, err := svc.ProcessPaidTransaction(ctx, orderCode, buildPayload(orderCode, 329_000))
	if err != nil {
		t.Fatalf("ProcessPaidTransaction: %v", err)
	}
	if result.AlreadyProcessed || result.Underpaid || result.UnknownOrder {
		t.Fatalf("unexpected result flags: %+v", result)
	}
	if result.UserID != userID {
		t.Fatalf("UserID: want %s, got %s", userID, result.UserID)
	}

	// Transaction status should be 'paid'.
	if st := getTxStatus(t, pool, txID); st != "paid" {
		t.Fatalf("tx status: want paid, got %s", st)
	}

	// Wallet should have 200 standard credits.
	std, prem := getWalletBalance(t, pool, userID)
	if std != 200 {
		t.Fatalf("standard_credits: want 200, got %d", std)
	}
	if prem != 0 {
		t.Fatalf("premium_credits: want 0, got %d", prem)
	}

	// Ledger should have a 'topup' row.
	if n := countLedgerByEventType(t, pool, userID, "topup"); n != 1 {
		t.Fatalf("topup ledger rows: want 1, got %d", n)
	}
}

func TestProcessPaidTransaction_IdempotentReplay(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	insertPendingTx(t, pool, userID, orderCode, "standard_pro_200")

	svc := newTestWebhookService(t, pool, nil)
	p := buildPayload(orderCode, 329_000)

	// First call succeeds.
	if _, err := svc.ProcessPaidTransaction(ctx, orderCode, p); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Second call must return AlreadyProcessed, wallet unchanged.
	result, err := svc.ProcessPaidTransaction(ctx, orderCode, p)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !result.AlreadyProcessed {
		t.Fatal("want AlreadyProcessed=true on replay, got false")
	}

	// Wallet still exactly 200 — not double-credited.
	std, _ := getWalletBalance(t, pool, userID)
	if std != 200 {
		t.Fatalf("idempotent replay: want 200 credits, got %d", std)
	}
}

func TestProcessPaidTransaction_ConcurrentRace_10x(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	insertPendingTx(t, pool, userID, orderCode, "standard_pro_200")

	svc := newTestWebhookService(t, pool, nil)
	p := buildPayload(orderCode, 329_000)

	const goroutines = 10
	var wg sync.WaitGroup
	errors := make([]error, goroutines)
	results := make([]service.ProcessResult, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i], errors[i] = svc.ProcessPaidTransaction(ctx, orderCode, p)
		}()
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Fatalf("goroutine %d error: %v", i, err)
		}
	}

	// Exactly one goroutine should have gotten a non-AlreadyProcessed result.
	grants := 0
	for _, r := range results {
		if !r.AlreadyProcessed {
			grants++
		}
	}
	if grants != 1 {
		t.Fatalf("want exactly 1 grant, got %d", grants)
	}

	// Wallet must have exactly 200 credits (not 10x).
	std, _ := getWalletBalance(t, pool, userID)
	if std != 200 {
		t.Fatalf("concurrent race: want 200 credits, got %d", std)
	}
}

func TestProcessPaidTransaction_UnknownOrder(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	svc := newTestWebhookService(t, pool, nil)
	// Order code that was never inserted.
	result, err := svc.ProcessPaidTransaction(ctx, "NOTEXIST0000", buildPayload("NOTEXIST0000", 329_000))
	if err != nil {
		t.Fatalf("UnknownOrder: %v", err)
	}
	if !result.UnknownOrder {
		t.Fatalf("want UnknownOrder=true, got %+v", result)
	}
}

func TestProcessPaidTransaction_Underpaid(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	txID := insertPendingTx(t, pool, userID, orderCode, "standard_pro_200") // needs 329k

	svc := newTestWebhookService(t, pool, nil)
	result, err := svc.ProcessPaidTransaction(ctx, orderCode, buildPayload(orderCode, 100_000)) // underpay
	if err != nil {
		t.Fatalf("Underpaid: %v", err)
	}
	if !result.Underpaid {
		t.Fatalf("want Underpaid=true, got %+v", result)
	}

	// Status must be 'manual_review'.
	if st := getTxStatus(t, pool, txID); st != "manual_review" {
		t.Fatalf("tx status: want manual_review, got %s", st)
	}

	// No credits granted.
	std, prem := getWalletBalance(t, pool, userID)
	if std != 0 || prem != 0 {
		t.Fatalf("underpaid: want 0/0 credits, got std=%d prem=%d", std, prem)
	}
}

func TestProcessPaidTransaction_Overpaid_ExactMatch(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	insertPendingTx(t, pool, userID, orderCode, "standard_pro_200") // 329k / 200 std

	svc := newTestWebhookService(t, pool, nil)
	// Exact match — diff = 0.
	result, err := svc.ProcessPaidTransaction(ctx, orderCode, buildPayload(orderCode, 329_000))
	if err != nil {
		t.Fatalf("ExactMatch: %v", err)
	}
	if result.Overpaid {
		t.Fatal("want Overpaid=false for exact match")
	}
	if result.BonusCredits != 0 {
		t.Fatalf("want BonusCredits=0, got %d", result.BonusCredits)
	}
	std, _ := getWalletBalance(t, pool, userID)
	if std != 200 {
		t.Fatalf("exact match: want 200 credits, got %d", std)
	}
}

func TestProcessPaidTransaction_Overpaid_SmallExcess(t *testing.T) {
	// standard_pro_200: 329k / 200 std → rate = 329000/200 = 1645 VND/credit
	// transferAmount = 400k → diff = 71k → bonus = floor(71000/1645) = 43
	pool := newTestPool(t)
	ctx := context.Background()

	alertCh := make(chan notify.AdminAlert, 10)
	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	insertPendingTx(t, pool, userID, orderCode, "standard_pro_200")

	svc := newTestWebhookService(t, pool, alertCh)
	result, err := svc.ProcessPaidTransaction(ctx, orderCode, buildPayload(orderCode, 400_000))
	if err != nil {
		t.Fatalf("Overpaid SmallExcess: %v", err)
	}
	if !result.Overpaid {
		t.Fatal("want Overpaid=true")
	}
	if result.BonusCredits != 43 {
		t.Fatalf("bonus: want 43, got %d", result.BonusCredits)
	}

	// Wallet: base 200 + bonus 43 = 243 standard credits.
	std, _ := getWalletBalance(t, pool, userID)
	if std != 243 {
		t.Fatalf("standard_credits: want 243, got %d", std)
	}

	// Ledger must have both 'topup' and 'topup_excess' rows.
	if n := countLedgerByEventType(t, pool, userID, "topup"); n != 1 {
		t.Fatalf("topup ledger rows: want 1, got %d", n)
	}
	if n := countLedgerByEventType(t, pool, userID, "topup_excess"); n != 1 {
		t.Fatalf("topup_excess ledger rows: want 1, got %d", n)
	}

	// Admin alert must have been sent (diff=71k >= 10k threshold).
	select {
	case alert := <-alertCh:
		if alert.Kind != "overpaid" {
			t.Fatalf("alert kind: want overpaid, got %s", alert.Kind)
		}
		if alert.BonusCredits != 43 {
			t.Fatalf("alert BonusCredits: want 43, got %d", alert.BonusCredits)
		}
	default:
		t.Fatal("expected admin alert for overpaid diff >= 10k, got none")
	}
}

func TestProcessPaidTransaction_Overpaid_LargeExcess(t *testing.T) {
	// diff = 380000 - 329000 = 51000 >= 50k → metadata.manual_review = true
	// bonus = floor(51000/1645) = 31
	pool := newTestPool(t)
	ctx := context.Background()

	alertCh := make(chan notify.AdminAlert, 10)
	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	txID := insertPendingTx(t, pool, userID, orderCode, "standard_pro_200")

	svc := newTestWebhookService(t, pool, alertCh)
	result, err := svc.ProcessPaidTransaction(ctx, orderCode, buildPayload(orderCode, 380_000))
	if err != nil {
		t.Fatalf("Overpaid LargeExcess: %v", err)
	}
	if result.BonusCredits != 31 {
		t.Fatalf("bonus: want 31, got %d", result.BonusCredits)
	}

	// metadata.manual_review must be true.
	var metaRaw []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT metadata FROM transactions WHERE id=$1`, txID,
	).Scan(&metaRaw); err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if !containsJSON(metaRaw, `"manual_review":true`) {
		t.Fatalf("metadata should contain manual_review:true, got: %s", metaRaw)
	}

	// Credits still granted.
	std, _ := getWalletBalance(t, pool, userID)
	if std != 231 { // 200 base + 31 bonus
		t.Fatalf("standard_credits: want 231, got %d", std)
	}
}

func TestProcessPaidTransaction_Q2_CancelRecovery(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	orderCode := randOrderCode()
	txID := insertPendingTx(t, pool, userID, orderCode, "standard_pro_200")

	// Cancel the transaction via CancelPendingTransaction.
	q := sqlcdb.New(pool)
	txSvc := service.NewTransactionService(pool, q, nil,
		&config.Config{SepayBankCode: "MBBank", SepayBankAccount: "123456789"},
		mustLogger(t),
	)
	if err := txSvc.CancelPendingTransaction(ctx, userID, txID, "test_cancel"); err != nil {
		t.Fatalf("CancelPendingTransaction: %v", err)
	}
	if st := getTxStatus(t, pool, txID); st != "cancelled" {
		t.Fatalf("after cancel: want cancelled, got %s", st)
	}

	// Late payment arrives — should recover.
	svc := newTestWebhookService(t, pool, nil)
	result, err := svc.ProcessPaidTransaction(ctx, orderCode, buildPayload(orderCode, 329_000))
	if err != nil {
		t.Fatalf("Q2 CancelRecovery: %v", err)
	}
	if !result.WasCancelled {
		t.Fatal("want WasCancelled=true")
	}

	if st := getTxStatus(t, pool, txID); st != "recovered_by_late_payment" {
		t.Fatalf("tx status: want recovered_by_late_payment, got %s", st)
	}

	// Credits granted.
	std, _ := getWalletBalance(t, pool, userID)
	if std != 200 {
		t.Fatalf("recovered credits: want 200, got %d", std)
	}
}

func TestProcessPaidTransaction_MixedCaseMemo(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	// Generate a random order code; use a known hex string for the lowercase test.
	orderCode := randOrderCode() // e.g. "A1B2C3D4E5F6"

	insertPendingTx(t, pool, userID, orderCode, "standard_pro_200")

	svc := newTestWebhookService(t, pool, nil)
	// Payload with lowercase memo — handler would ToUpper before calling here.
	lowerContent := "sbf topup " + strings.ToLower(orderCode)
	p := sepay.Payload{
		Gateway:        "MBBank",
		AccountNumber:  "123456789",
		Content:        lowerContent, // lowercase memo
		TransferType:   "in",
		TransferAmount: 329_000,
	}
	// Simulate handler's ToUpper normalization — handler always calls strings.ToUpper(match[1]).
	normalizedCode := strings.ToUpper(sepay.OrderCodeRe.FindStringSubmatch(p.Content)[1])
	result, err := svc.ProcessPaidTransaction(ctx, normalizedCode, p)
	if err != nil {
		t.Fatalf("MixedCaseMemo: %v", err)
	}
	if result.AlreadyProcessed || result.UnknownOrder {
		t.Fatalf("want success, got %+v", result)
	}
	std, _ := getWalletBalance(t, pool, userID)
	if std != 200 {
		t.Fatalf("mixed case: want 200 credits, got %d", std)
	}
}

// ─────────────────────────── Helper utilities ──────────────────────────────

// containsJSON checks if raw JSON bytes contain a key-value pair.
// Handles both compact ("key":value) and spaced ("key": value) JSON formatting.
func containsJSON(raw []byte, sub string) bool {
	s := string(raw)
	// Try exact match first.
	if strings.Contains(s, sub) {
		return true
	}
	// Try with space after colon (Postgres jsonb pretty-formats with spaces).
	spacedSub := strings.ReplaceAll(sub, `":`, `": `)
	return strings.Contains(s, spacedSub)
}

// mustLogger returns a zap.Logger that fails the test if logger creation fails.
func mustLogger(t *testing.T) *zap.Logger {
	t.Helper()
	log, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("zap.NewDevelopment: %v", err)
	}
	return log
}
