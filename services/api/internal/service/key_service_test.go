// Package service_test — integration tests for KeyService using live Postgres.
// Requires DATABASE_URL env var. Each test uses unique user IDs for isolation.
// Tables are NOT truncated; unique users prevent inter-test collisions.
package service_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── Test harness helpers ──────────────────────────

func newTestKeyService(t *testing.T, pool *pgxpool.Pool) *service.KeyService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	return service.NewKeyService(pool, log)
}

// insertTestUser creates a minimal user row and wallet so FK constraints pass.
// Returns the new user UUID.
func insertTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tgID := uniqueTgID()

	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (telegram_id) VALUES ($1) RETURNING id`, tgID,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insertTestUser: %v", err)
	}

	// Ensure wallet row exists (FK from api_keys via user).
	_, err = pool.Exec(ctx,
		`INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID,
	)
	if err != nil {
		t.Fatalf("insertTestUser wallet: %v", err)
	}
	return userID
}

// ─────────────────────────── Tests ─────────────────────────────────────────

// TestIssue_NewKey verifies a fresh user gets one active api_key row after Issue.
func TestIssue_NewKey(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestKeyService(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	plaintext, prefix, err := svc.Issue(ctx, userID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if plaintext == "" {
		t.Fatal("plaintext must not be empty")
	}
	if len(prefix) != 12 {
		t.Fatalf("prefix length = %d, want 12; got %q", len(prefix), prefix)
	}
	if !strings.HasPrefix(plaintext, prefix) {
		t.Fatalf("plaintext %q does not start with prefix %q", plaintext, prefix)
	}

	// Verify exactly 1 active row for this user.
	var activeCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE user_id=$1 AND is_active=TRUE`, userID,
	).Scan(&activeCount)
	if err != nil {
		t.Fatalf("active count query: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active key row, got %d", activeCount)
	}

	// Verify key_prefix stored matches returned prefix.
	var storedPrefix string
	err = pool.QueryRow(ctx,
		`SELECT key_prefix FROM api_keys WHERE user_id=$1 AND is_active=TRUE`, userID,
	).Scan(&storedPrefix)
	if err != nil {
		t.Fatalf("stored prefix query: %v", err)
	}
	if storedPrefix != prefix {
		t.Fatalf("stored prefix %q != returned prefix %q", storedPrefix, prefix)
	}
}

// TestIssue_RevokesOldActive issues twice and checks the old key is revoked.
func TestIssue_RevokesOldActive(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestKeyService(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	// First issue.
	_, _, err := svc.Issue(ctx, userID)
	if err != nil {
		t.Fatalf("first Issue: %v", err)
	}

	// Capture first key hash before second issue.
	var firstHash []byte
	err = pool.QueryRow(ctx,
		`SELECT key_hash FROM api_keys WHERE user_id=$1 AND is_active=TRUE`, userID,
	).Scan(&firstHash)
	if err != nil {
		t.Fatalf("first key hash query: %v", err)
	}

	// Second issue — should revoke first.
	_, _, err = svc.Issue(ctx, userID)
	if err != nil {
		t.Fatalf("second Issue: %v", err)
	}

	// Old key must be inactive with revoked_at set.
	var isActive bool
	var revokedAtNull bool
	err = pool.QueryRow(ctx,
		`SELECT is_active, revoked_at IS NULL FROM api_keys WHERE key_hash=$1`, firstHash,
	).Scan(&isActive, &revokedAtNull)
	if err != nil {
		t.Fatalf("old key state query: %v", err)
	}
	if isActive {
		t.Fatal("old key should be is_active=FALSE after second Issue")
	}
	if revokedAtNull {
		t.Fatal("old key revoked_at should be non-NULL")
	}

	// Exactly 1 active key remains.
	var activeCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE user_id=$1 AND is_active=TRUE`, userID,
	).Scan(&activeCount)
	if err != nil {
		t.Fatalf("active count: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active key, got %d", activeCount)
	}
}

// TestIssue_HashMatchesPlaintext verifies SHA-256(plaintext) == stored key_hash.
func TestIssue_HashMatchesPlaintext(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestKeyService(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	plaintext, _, err := svc.Issue(ctx, userID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Load stored hash from DB.
	var storedHash []byte
	err = pool.QueryRow(ctx,
		`SELECT key_hash FROM api_keys WHERE user_id=$1 AND is_active=TRUE`, userID,
	).Scan(&storedHash)
	if err != nil {
		t.Fatalf("stored hash query: %v", err)
	}

	expected := sha256.Sum256([]byte(plaintext))
	if string(storedHash) != string(expected[:]) {
		t.Fatalf("stored hash does not match SHA-256(plaintext)")
	}
}

// TestGetActiveMasked_Exists verifies masked display format after Issue.
func TestGetActiveMasked_Exists(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestKeyService(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	plaintext, prefix, err := svc.Issue(ctx, userID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	_ = plaintext // not used after this point — discard per security policy

	masked, exists, err := svc.GetActiveMasked(ctx, userID)
	if err != nil {
		t.Fatalf("GetActiveMasked: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true after Issue")
	}
	if !strings.HasPrefix(masked, prefix) {
		t.Fatalf("masked %q does not start with prefix %q", masked, prefix)
	}
	if !strings.Contains(masked, "•••••") {
		t.Fatalf("masked %q does not contain bullet separator", masked)
	}
	// Total: 12 prefix + 5 bullets + 4 suffix = 21 display chars
	if len([]rune(masked)) != 21 {
		t.Fatalf("masked rune length = %d, want 21; got %q", len([]rune(masked)), masked)
	}
}

// TestGetActiveMasked_NotExists returns (false, nil) for a user with no key.
func TestGetActiveMasked_NotExists(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestKeyService(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	masked, exists, err := svc.GetActiveMasked(ctx, userID)
	if err != nil {
		t.Fatalf("GetActiveMasked: unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false for user with no key")
	}
	if masked != "" {
		t.Fatalf("expected empty masked, got %q", masked)
	}
}

// TestIssue_ConcurrentRace_ExactlyOneActive spawns 10 goroutines all calling
// Issue for the same user simultaneously. Asserts the §1.3 invariant:
// exactly 1 active key remains after all goroutines finish.
//
// Run with: go test -race -count=10 -run TestIssue_ConcurrentRace_ExactlyOneActive
//
// Expected outcomes per goroutine:
//   - Success (plaintext returned): at least 1 must succeed.
//   - ErrKeyRaceContention: acceptable when retry budget exhausted under high contention.
//   - Other errors: fail the test — unexpected failure mode.
func TestIssue_ConcurrentRace_ExactlyOneActive(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestKeyService(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	const goroutines = 10
	type result struct {
		plaintext string
		err       error
	}
	results := make([]result, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			pt, _, err := svc.Issue(ctx, userID)
			results[idx] = result{plaintext: pt, err: err}
		}(i)
	}
	wg.Wait()

	// Give best-effort audit goroutines a moment to finish.
	time.Sleep(150 * time.Millisecond)

	// Tally outcomes.
	var successCount, contentionCount, otherErrCount int
	for i, r := range results {
		switch {
		case r.err == nil:
			successCount++
			if r.plaintext == "" {
				t.Errorf("goroutine %d: success but empty plaintext", i)
			}
		case errors.Is(r.err, service.ErrKeyRaceContention):
			contentionCount++
			t.Logf("goroutine %d: ErrKeyRaceContention (expected under contention)", i)
		default:
			otherErrCount++
			t.Errorf("goroutine %d: unexpected error: %v", i, r.err)
		}
	}

	if successCount == 0 {
		t.Fatal("at least 1 goroutine must succeed; all returned errors")
	}
	t.Logf("outcomes: success=%d contention=%d other=%d", successCount, contentionCount, otherErrCount)

	// §1.3 invariant: exactly 1 active key for this user.
	var activeCount int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE user_id=$1 AND is_active=TRUE`, userID,
	).Scan(&activeCount)
	if err != nil {
		t.Fatalf("active count query: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("§1.3 violated: expected exactly 1 active key after concurrent Issue, got %d", activeCount)
	}

	// Total rows for this user = goroutines that reached InsertKey (success + some contention
	// attempts that completed before race). Revoked rows must have revoked_at set.
	var revokedWithoutTimestamp int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE user_id=$1 AND is_active=FALSE AND revoked_at IS NULL`,
		userID,
	).Scan(&revokedWithoutTimestamp)
	if err != nil {
		t.Fatalf("revoked_at null check: %v", err)
	}
	if revokedWithoutTimestamp > 0 {
		t.Errorf("found %d inactive keys with NULL revoked_at — revoke logic incomplete", revokedWithoutTimestamp)
	}
}
