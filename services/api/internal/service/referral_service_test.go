// referral_service_test.go — Phase 07: integration tests for ReferralService.
// Requires DATABASE_URL env var (live Postgres with Phase 02 schema).
// Each test uses fresh user IDs for isolation; no table truncation.
package service_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── harness ──────────────────────────────────────

func newTestReferralService(t *testing.T, pool *pgxpool.Pool) *service.ReferralService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	return service.NewReferralService(pool, q, log.Named("referral_test"))
}

// ─────────────────────────── tests ────────────────────────────────────────

// TestEnsureCode_NewUser creates a code for a new user and verifies it is 6 chars.
func TestEnsureCode_NewUser(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestReferralService(t, pool)
	userID := insertTestUser(t, pool)

	code, err := svc.EnsureCode(context.Background(), userID)
	if err != nil {
		t.Fatalf("EnsureCode: unexpected error: %v", err)
	}
	if len(code) < 6 {
		t.Errorf("EnsureCode: expected >= 6 chars, got %q (len=%d)", code, len(code))
	}
}

// TestEnsureCode_Idempotent verifies two calls for the same user return the same code.
func TestEnsureCode_Idempotent(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestReferralService(t, pool)
	userID := insertTestUser(t, pool)

	code1, err := svc.EnsureCode(context.Background(), userID)
	if err != nil {
		t.Fatalf("EnsureCode first call: %v", err)
	}
	code2, err := svc.EnsureCode(context.Background(), userID)
	if err != nil {
		t.Fatalf("EnsureCode second call: %v", err)
	}
	if code1 != code2 {
		t.Errorf("EnsureCode not idempotent: first=%q second=%q", code1, code2)
	}
}

// TestEnsureCode_Uniqueness generates 50 codes for distinct users and checks for collisions.
// 33^6 ≈ 1.3B space → collision probability at 50 samples is negligible.
func TestEnsureCode_Uniqueness(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestReferralService(t, pool)

	seen := make(map[string]struct{}, 50)
	for i := 0; i < 50; i++ {
		userID := insertTestUser(t, pool)
		code, err := svc.EnsureCode(context.Background(), userID)
		if err != nil {
			t.Fatalf("iteration %d: EnsureCode: %v", i, err)
		}
		if _, dup := seen[code]; dup {
			t.Errorf("duplicate code %q at iteration %d", code, i)
		}
		seen[code] = struct{}{}
	}
}

// TestProcessReferralOnStart_Valid verifies attribution: referred_by set + count incremented.
func TestProcessReferralOnStart_Valid(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestReferralService(t, pool)

	referrerID := insertTestUser(t, pool)
	newUserID := insertTestUser(t, pool)

	// Create referrer's code.
	code, err := svc.EnsureCode(context.Background(), referrerID)
	if err != nil {
		t.Fatalf("EnsureCode: %v", err)
	}

	// Process referral for new user.
	if err := svc.ProcessReferralOnStart(context.Background(), newUserID, code); err != nil {
		t.Fatalf("ProcessReferralOnStart: %v", err)
	}

	// Verify users.referred_by is set.
	var referredBy *string
	err = pool.QueryRow(context.Background(),
		`SELECT referred_by::text FROM users WHERE id = $1`, newUserID,
	).Scan(&referredBy)
	if err != nil {
		t.Fatalf("query referred_by: %v", err)
	}
	if referredBy == nil {
		t.Fatal("referred_by should be set, got NULL")
	}

	// Verify referral count incremented.
	q := sqlcdb.New(pool)
	ref, err := q.GetReferralByUser(context.Background(), referrerID)
	if err != nil {
		t.Fatalf("GetReferralByUser: %v", err)
	}
	if ref.TotalReferred != 1 {
		t.Errorf("total_referred: got %d, want 1", ref.TotalReferred)
	}
}

// TestProcessReferralOnStart_Invalid verifies no error and no DB change for a garbage code.
func TestProcessReferralOnStart_Invalid(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestReferralService(t, pool)
	newUserID := insertTestUser(t, pool)

	err := svc.ProcessReferralOnStart(context.Background(), newUserID, "XXXXXX")
	if err != nil {
		t.Errorf("invalid code should be silent no-op, got error: %v", err)
	}

	// referred_by must still be NULL.
	var referredBy *string
	_ = pool.QueryRow(context.Background(),
		`SELECT referred_by::text FROM users WHERE id = $1`, newUserID,
	).Scan(&referredBy)
	if referredBy != nil {
		t.Errorf("referred_by should remain NULL for invalid code, got %v", *referredBy)
	}
}

// TestProcessReferralOnStart_OwnCode verifies a user cannot refer themselves.
func TestProcessReferralOnStart_OwnCode(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestReferralService(t, pool)
	userID := insertTestUser(t, pool)

	code, err := svc.EnsureCode(context.Background(), userID)
	if err != nil {
		t.Fatalf("EnsureCode: %v", err)
	}

	// Attempt self-referral.
	err = svc.ProcessReferralOnStart(context.Background(), userID, code)
	if err != nil {
		t.Errorf("own-code self-referral should be silent no-op, got error: %v", err)
	}

	// referred_by must remain NULL.
	var referredBy *string
	_ = pool.QueryRow(context.Background(),
		`SELECT referred_by::text FROM users WHERE id = $1`, userID,
	).Scan(&referredBy)
	if referredBy != nil {
		t.Errorf("referred_by should remain NULL after own-code attempt, got %v", *referredBy)
	}
}

// TestProcessReferralOnStart_Idempotent verifies write-once: second call is a no-op.
func TestProcessReferralOnStart_Idempotent(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestReferralService(t, pool)

	referrerID := insertTestUser(t, pool)
	newUserID := insertTestUser(t, pool)

	code, err := svc.EnsureCode(context.Background(), referrerID)
	if err != nil {
		t.Fatalf("EnsureCode: %v", err)
	}

	// First call — should attribute.
	if err := svc.ProcessReferralOnStart(context.Background(), newUserID, code); err != nil {
		t.Fatalf("first ProcessReferralOnStart: %v", err)
	}

	// Second call — should be a no-op (WHERE referred_by IS NULL guard).
	if err := svc.ProcessReferralOnStart(context.Background(), newUserID, code); err != nil {
		t.Fatalf("second ProcessReferralOnStart: %v", err)
	}

	// Count should still be 1, not 2.
	q := sqlcdb.New(pool)
	ref, err := q.GetReferralByUser(context.Background(), referrerID)
	if err != nil {
		t.Fatalf("GetReferralByUser: %v", err)
	}
	if ref.TotalReferred != 1 {
		t.Errorf("idempotent second call should not double-count: got %d, want 1", ref.TotalReferred)
	}
}
