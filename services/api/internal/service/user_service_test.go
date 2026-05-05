// Package service_test — integration tests for UserService using live Postgres.
//
// Test harness: live Postgres via DATABASE_URL env var (from .env or CI).
// testcontainers-go deferred: Windows Docker Desktop fragility + added weight.
// Each test uses a unique Telegram ID to avoid inter-test pollution.
// Tables are NOT truncated between tests — unique tgIDs are used for isolation.
//
// Concurrent race test (2 tg_ids same phone → exactly 1 succeeds) is deferred to
// phase-10 Suite B per spec. Noted here for future reference.
package service_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── Mock KeyIssuer ────────────────────────────────

type mockKeyIssuer struct {
	plaintext string
	prefix    string
	err       error
}

func (m *mockKeyIssuer) Issue(_ context.Context, _ uuid.UUID) (string, string, error) {
	return m.plaintext, m.prefix, m.err
}

var goodKeyIssuer = &mockKeyIssuer{
	plaintext: "sbf_live_TESTKEY",
	prefix:    "sbf_live_TES",
}

// ─────────────────────────── Test harness ──────────────────────────────────

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// Load .env if present (local dev); CI exports DATABASE_URL directly.
	_ = godotenv.Load("../../.env") // relative to this file's package dir

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("pool.Ping: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newTestService(t *testing.T, pool *pgxpool.Pool, keys service.KeyIssuer) *service.UserService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	if keys == nil {
		keys = service.NoopKeyIssuer
	}
	return service.New(pool, nil, keys, log)
}

// uniqueTgID returns a random Telegram ID in range [1_000_000, 2_000_000) for isolation.
func uniqueTgID() int64 {
	return int64(1_000_000 + rand.Intn(1_000_000)) //nolint:gosec
}

// uniquePhone returns a valid-enough VN phone that won't collide with other tests.
func uniquePhone() string {
	// Use 035xxxxxxx range with random 7 suffix digits.
	suffix := rand.Intn(10_000_000) //nolint:gosec
	return fmt.Sprintf("035%07d", suffix)
}

// ─────────────────────────── Tests ─────────────────────────────────────────

func TestEnsureStub_NewUser(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestService(t, pool, nil)
	ctx := context.Background()

	tgID := uniqueTgID()
	user, err := svc.EnsureStub(ctx, tgID, "testuser", "Test")
	if err != nil {
		t.Fatalf("EnsureStub: %v", err)
	}
	if user.ID == (uuid.UUID{}) {
		t.Fatal("expected non-zero UUID")
	}
	if user.TelegramID != tgID {
		t.Fatalf("TelegramID mismatch: got %d want %d", user.TelegramID, tgID)
	}

	// Verify wallet row was created.
	var walletExists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM wallets WHERE user_id=$1)`, user.ID,
	).Scan(&walletExists)
	if err != nil {
		t.Fatalf("wallet check: %v", err)
	}
	if !walletExists {
		t.Fatal("wallet row not created")
	}
}

func TestEnsureStub_ExistingUser(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestService(t, pool, nil)
	ctx := context.Background()

	tgID := uniqueTgID()

	// First call: creates the user.
	u1, err := svc.EnsureStub(ctx, tgID, "original", "Orig")
	if err != nil {
		t.Fatalf("first EnsureStub: %v", err)
	}

	// Second call: updates last_active_at, no duplicate wallet.
	u2, err := svc.EnsureStub(ctx, tgID, "updated", "Upd")
	if err != nil {
		t.Fatalf("second EnsureStub: %v", err)
	}
	if u1.ID != u2.ID {
		t.Fatalf("IDs differ: %s vs %s", u1.ID, u2.ID)
	}

	// Wallet count must still be exactly 1.
	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM wallets WHERE user_id=$1`, u1.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("wallet count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 wallet row, got %d", count)
	}
}

func TestVerifyContactAndGrantTrial_Success(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestService(t, pool, goodKeyIssuer)
	ctx := context.Background()

	tgID := uniqueTgID()
	user, err := svc.EnsureStub(ctx, tgID, "trialuser", "Trial")
	if err != nil {
		t.Fatalf("EnsureStub: %v", err)
	}

	phone := uniquePhone()
	plaintext, prefix, err := svc.VerifyContactAndGrantTrial(ctx, user.ID, phone)
	if err != nil {
		t.Fatalf("VerifyContactAndGrantTrial: %v", err)
	}
	if plaintext != "sbf_live_TESTKEY" {
		t.Fatalf("plaintext: got %q", plaintext)
	}
	if prefix != "sbf_live_TES" {
		t.Fatalf("prefix: got %q", prefix)
	}

	// Verify wallet.standard_credits == 5.
	var credits int32
	err = pool.QueryRow(ctx,
		`SELECT standard_credits FROM wallets WHERE user_id=$1`, user.ID,
	).Scan(&credits)
	if err != nil {
		t.Fatalf("wallet credits: %v", err)
	}
	if credits != 5 {
		t.Fatalf("expected 5 standard_credits, got %d", credits)
	}

	// Verify trial_used = TRUE.
	var trialUsed bool
	err = pool.QueryRow(ctx,
		`SELECT trial_used FROM users WHERE id=$1`, user.ID,
	).Scan(&trialUsed)
	if err != nil {
		t.Fatalf("trial_used check: %v", err)
	}
	if !trialUsed {
		t.Fatal("expected trial_used=TRUE")
	}

	// Verify ledger row inserted with event_type='trial_grant'.
	var ledgerCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger WHERE user_id=$1 AND event_type='trial_grant'`, user.ID,
	).Scan(&ledgerCount)
	if err != nil {
		t.Fatalf("ledger check: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("expected 1 ledger row, got %d", ledgerCount)
	}
}

func TestVerifyContactAndGrantTrial_AlreadyUsed(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestService(t, pool, goodKeyIssuer)
	ctx := context.Background()

	tgID := uniqueTgID()
	user, _ := svc.EnsureStub(ctx, tgID, "dupeuser", "Dupe")
	phone := uniquePhone()

	// First call: succeeds.
	_, _, err := svc.VerifyContactAndGrantTrial(ctx, user.ID, phone)
	if err != nil {
		t.Fatalf("first trial: %v", err)
	}

	// Second call: same user — must return ErrTrialAlreadyUsed.
	_, _, err = svc.VerifyContactAndGrantTrial(ctx, user.ID, phone)
	if !errors.Is(err, service.ErrTrialAlreadyUsed) {
		t.Fatalf("expected ErrTrialAlreadyUsed, got %v", err)
	}

	// No new ledger row — credits stay 5.
	var ledgerCount int
	pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger WHERE user_id=$1 AND event_type='trial_grant'`, user.ID,
	).Scan(&ledgerCount) //nolint:errcheck
	if ledgerCount != 1 {
		t.Fatalf("expected exactly 1 ledger row, got %d", ledgerCount)
	}
}

func TestVerifyContactAndGrantTrial_PhoneReused(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestService(t, pool, goodKeyIssuer)
	ctx := context.Background()

	// User 1: claims trial with a specific phone.
	tgID1 := uniqueTgID()
	u1, _ := svc.EnsureStub(ctx, tgID1, "winner", "Win")
	phone := uniquePhone()

	_, _, err := svc.VerifyContactAndGrantTrial(ctx, u1.ID, phone)
	if err != nil {
		t.Fatalf("user1 trial: %v", err)
	}

	// User 2: different tg_id, SAME phone → ErrTrialPhoneReused.
	tgID2 := uniqueTgID()
	u2, _ := svc.EnsureStub(ctx, tgID2, "loser", "Los")

	_, _, err = svc.VerifyContactAndGrantTrial(ctx, u2.ID, phone)
	if !errors.Is(err, service.ErrTrialPhoneReused) {
		t.Fatalf("expected ErrTrialPhoneReused, got %v", err)
	}

	// Winner's credits intact (5).
	var credits int32
	pool.QueryRow(ctx,
		`SELECT standard_credits FROM wallets WHERE user_id=$1`, u1.ID,
	).Scan(&credits) //nolint:errcheck
	if credits != 5 {
		t.Fatalf("winner credits should be 5, got %d", credits)
	}

	// Loser's credits still 0.
	pool.QueryRow(ctx,
		`SELECT standard_credits FROM wallets WHERE user_id=$1`, u2.ID,
	).Scan(&credits) //nolint:errcheck
	if credits != 0 {
		t.Fatalf("loser credits should be 0, got %d", credits)
	}
}

func TestVerifyContactAndGrantTrial_Banned(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestService(t, pool, goodKeyIssuer)
	ctx := context.Background()

	tgID := uniqueTgID()
	user, _ := svc.EnsureStub(ctx, tgID, "banned", "Ban")

	// Set is_banned=TRUE directly.
	_, err := pool.Exec(ctx, `UPDATE users SET is_banned=TRUE WHERE id=$1`, user.ID)
	if err != nil {
		t.Fatalf("ban user: %v", err)
	}

	_, _, err = svc.VerifyContactAndGrantTrial(ctx, user.ID, uniquePhone())
	if !errors.Is(err, service.ErrTrialUserBanned) {
		t.Fatalf("expected ErrTrialUserBanned, got %v", err)
	}

	// No state change: trial_used still FALSE.
	var trialUsed bool
	pool.QueryRow(ctx,
		`SELECT trial_used FROM users WHERE id=$1`, user.ID,
	).Scan(&trialUsed) //nolint:errcheck
	if trialUsed {
		t.Fatal("banned user should not have trial_used set")
	}
}

// NOTE: Concurrent race test (2 tg_ids same phone → exactly 1 succeeds, 1 gets
// ErrTrialPhoneReused) is deferred to phase-10 Suite B per spec.
// Rationale: the partial unique index idx_users_phone_trial guarantees at-most-one
// winner by construction — no pre-check needed. The race test validates index
// semantics under concurrent load, which requires a controlled goroutine harness.
