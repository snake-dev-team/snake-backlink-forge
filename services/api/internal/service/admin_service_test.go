// admin_service_test.go — integration tests for AdminService using live Postgres.
// Requires DATABASE_URL env var. Uses service_test package.
// Tests cover: Stats, Grant (F5 atomicity), Ban (M3 guard), Unban, Lookup.
package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── Harness ───────────────────────────────────────

func newAdminSvc(t *testing.T, pool *pgxpool.Pool, adminIDs []int64) *service.AdminService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	audit := service.NewAuditService(pool, q, log.Named("audit"))
	cfg := &config.Config{AdminTelegramIDs: adminIDs}
	return service.NewAdminService(pool, q, audit, cfg, log.Named("admin"))
}

// countAuditRows returns the number of audit_log rows for a given user_id + event.
func countAuditRows(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, event string) int64 {
	t.Helper()
	var n int64
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND event = $2`, userID, event,
	).Scan(&n)
	if err != nil {
		t.Fatalf("countAuditRows: %v", err)
	}
	return n
}

// walletCredits returns (premium, standard) for a user.
func walletCredits(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) (premium, standard int32) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT premium_credits, standard_credits FROM wallets WHERE user_id = $1`, userID,
	).Scan(&premium, &standard)
	if err != nil {
		t.Fatalf("walletCredits: %v", err)
	}
	return
}

// ─────────────────────────── Stats ─────────────────────────────────────────

// TestStats_EmptyDB: Stats on a fresh DB returns non-negative counts (other tests may
// have added rows, but Stats must return without error and all int64 fields ≥ 0).
func TestStats_EmptyDB(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, nil)

	st, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Users < 0 || st.ActiveKeys < 0 || st.PendingTx < 0 {
		t.Fatalf("negative stats: %+v", st)
	}
}

// ─────────────────────────── Grant (F5) ────────────────────────────────────

// TestGrant_F5_Happy: admin grants 100 standard credits → wallet +100, audit row with ledger_id.
func TestGrant_F5_Happy(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, []int64{9999})
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	const adminTGID = int64(9999)

	newBal, err := svc.Grant(ctx, adminTGID, userID, "standard", 100, "test_grant")
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if newBal != 100 {
		t.Fatalf("expected new balance 100, got %d", newBal)
	}

	// Wallet must reflect the grant.
	_, std := walletCredits(t, pool, userID)
	if std != 100 {
		t.Fatalf("wallet standard_credits = %d, want 100", std)
	}

	// Audit row must exist with a populated ledger_id in metadata.
	var metaRaw []byte
	err = pool.QueryRow(ctx,
		`SELECT metadata FROM audit_log WHERE user_id = $1 AND event = 'admin_grant' ORDER BY id DESC LIMIT 1`,
		userID,
	).Scan(&metaRaw)
	if err != nil {
		t.Fatalf("audit row query: %v", err)
	}
	if len(metaRaw) == 0 {
		t.Fatal("audit metadata is empty")
	}
	metaStr := string(metaRaw)
	if !contains(metaStr, "ledger_id") {
		t.Fatalf("audit metadata missing ledger_id: %s", metaStr)
	}
	if !contains(metaStr, "test_grant") {
		t.Fatalf("audit metadata missing reason: %s", metaStr)
	}
}

// TestGrant_F5_RollbackOnGrantFail: non-existent target_user_id → grant_credits FK error
// → rollback → no audit row, wallet unchanged.
func TestGrant_F5_RollbackOnGrantFail(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, []int64{9999})
	ctx := context.Background()

	nonExistentUserID := uuid.New() // no matching users row

	_, err := svc.Grant(ctx, 9999, nonExistentUserID, "standard", 50, "rollback_test")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}

	// No audit row should exist for this user_id.
	n := countAuditRows(t, pool, nonExistentUserID, "admin_grant")
	if n != 0 {
		t.Fatalf("expected 0 audit rows after grant fail, got %d", n)
	}
}

// TestGrant_F5_AmountValidation: amount=0 → ErrAdminGrantAmountOOB; amount=10001 → same.
func TestGrant_F5_AmountValidation(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, nil)
	ctx := context.Background()
	userID := insertTestUser(t, pool)

	for _, bad := range []int{0, -1, 10001} {
		_, err := svc.Grant(ctx, 1, userID, "standard", bad, "test")
		if err != service.ErrAdminGrantAmountOOB {
			t.Fatalf("amount=%d: expected ErrAdminGrantAmountOOB, got %v", bad, err)
		}
	}
}

// TestGrant_F5_InvalidPool: pool="invalid" → ErrAdminGrantInvalidPool.
func TestGrant_F5_InvalidPool(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, nil)
	ctx := context.Background()
	userID := insertTestUser(t, pool)

	_, err := svc.Grant(ctx, 1, userID, "invalid", 10, "test")
	if err != service.ErrAdminGrantInvalidPool {
		t.Fatalf("expected ErrAdminGrantInvalidPool, got %v", err)
	}
}

// TestGrant_F5_RollbackOnAuditFail: inject a Postgres trigger that fails audit_log INSERT
// when metadata contains sentinel → rollback → wallet unchanged, no ledger row added.
func TestGrant_F5_RollbackOnAuditFail(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, []int64{9999})
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	_, stdBefore := walletCredits(t, pool, userID)

	// Install test trigger that rejects INSERT when metadata contains "FAIL_SENTINEL".
	_, err := pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION test_audit_fail_fn() RETURNS trigger AS $$
		BEGIN
			IF NEW.metadata::text LIKE '%FAIL_SENTINEL%' THEN
				RAISE EXCEPTION 'test trigger fail';
			END IF;
			RETURN NEW;
		END $$ LANGUAGE plpgsql;
	`)
	if err != nil {
		t.Fatalf("create trigger function: %v", err)
	}
	_, err = pool.Exec(ctx, `
		CREATE TRIGGER test_audit_fail_trg
		BEFORE INSERT ON audit_log
		FOR EACH ROW EXECUTE FUNCTION test_audit_fail_fn();
	`)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_audit_fail_trg ON audit_log;`)
		_, _ = pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS test_audit_fail_fn();`)
	})

	// Grant with sentinel reason — trigger will reject audit INSERT → rollback.
	_, grantErr := svc.Grant(ctx, 9999, userID, "standard", 50, "FAIL_SENTINEL")
	if grantErr == nil {
		t.Fatal("expected error from audit trigger, got nil")
	}

	// Wallet must be unchanged.
	_, stdAfter := walletCredits(t, pool, userID)
	if stdAfter != stdBefore {
		t.Fatalf("wallet changed after rollback: before=%d after=%d", stdBefore, stdAfter)
	}

	// No audit row for this user with admin_grant event.
	n := countAuditRows(t, pool, userID, "admin_grant")
	if n != 0 {
		t.Fatalf("expected 0 audit rows after trigger-induced rollback, got %d", n)
	}

	// No ledger row for this user from this grant attempt.
	var ledgerN int64
	_ = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger WHERE user_id = $1 AND event_type = 'admin_adjust'`, userID,
	).Scan(&ledgerN)
	if ledgerN != 0 {
		t.Fatalf("expected 0 ledger rows after rollback, got %d", ledgerN)
	}
}

// ─────────────────────────── Ban (M3) ──────────────────────────────────────

// TestBan_M3_SelfBanBlocked: admin tries to ban an admin tg_id → ErrCannotBanAdmin, DB unchanged.
func TestBan_M3_SelfBanBlocked(t *testing.T) {
	pool := newTestPool(t)
	adminTGID := uniqueTgID() // unique per run to avoid collision
	svc := newAdminSvc(t, pool, []int64{adminTGID})
	ctx := context.Background()

	// Create a user row for the admin tg_id.
	var adminUserID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (telegram_id) VALUES ($1) RETURNING id`, adminTGID,
	).Scan(&adminUserID)
	if err != nil {
		t.Fatalf("insert admin user: %v", err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, adminUserID)

	banErr := svc.Ban(ctx, adminTGID, adminTGID, "self_ban_attempt") //nolint:gosec
	if banErr != service.ErrCannotBanAdmin {
		t.Fatalf("expected ErrCannotBanAdmin, got %v", banErr)
	}

	// Verify is_banned unchanged (still false).
	var isBanned bool
	_ = pool.QueryRow(ctx,
		`SELECT is_banned FROM users WHERE telegram_id = $1`, adminTGID,
	).Scan(&isBanned)
	if isBanned {
		t.Fatal("is_banned must remain false after blocked ban attempt")
	}
}

// TestBan_Normal: admin bans a regular user → is_banned=true, audit row.
func TestBan_Normal(t *testing.T) {
	pool := newTestPool(t)
	const adminTGID = int64(88888)
	svc := newAdminSvc(t, pool, []int64{adminTGID})
	ctx := context.Background()

	// Create target user with a unique tg_id.
	targetTGID := uniqueTgID()
	var targetUserID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (telegram_id) VALUES ($1) RETURNING id`, targetTGID,
	).Scan(&targetUserID)
	if err != nil {
		t.Fatalf("insert target user: %v", err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, targetUserID)

	if err := svc.Ban(ctx, adminTGID, targetTGID, "test_ban"); err != nil {
		t.Fatalf("Ban: %v", err)
	}

	var isBanned bool
	_ = pool.QueryRow(ctx,
		`SELECT is_banned FROM users WHERE telegram_id = $1`, targetTGID,
	).Scan(&isBanned)
	if !isBanned {
		t.Fatal("expected is_banned=true after ban")
	}

	n := countAuditRows(t, pool, targetUserID, "admin_ban")
	if n == 0 {
		t.Fatal("expected audit row for admin_ban")
	}
}

// TestUnban_Normal: ban then unban → is_banned=false.
func TestUnban_Normal(t *testing.T) {
	pool := newTestPool(t)
	const adminTGID = int64(99991)
	svc := newAdminSvc(t, pool, []int64{adminTGID})
	ctx := context.Background()

	targetTGID := uniqueTgID()
	var targetUserID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (telegram_id) VALUES ($1) RETURNING id`, targetTGID,
	).Scan(&targetUserID)
	if err != nil {
		t.Fatalf("insert target: %v", err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, targetUserID)

	if err := svc.Ban(ctx, adminTGID, targetTGID, "ban_for_unban_test"); err != nil {
		t.Fatalf("Ban: %v", err)
	}
	if err := svc.Unban(ctx, adminTGID, targetTGID, "unban_test"); err != nil {
		t.Fatalf("Unban: %v", err)
	}

	var isBanned bool
	_ = pool.QueryRow(ctx,
		`SELECT is_banned FROM users WHERE telegram_id = $1`, targetTGID,
	).Scan(&isBanned)
	if isBanned {
		t.Fatal("expected is_banned=false after unban")
	}
}

// ─────────────────────────── Lookup ────────────────────────────────────────

// TestLookup_ByTelegramID: lookup by numeric string returns the correct user.
func TestLookup_ByTelegramID(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, nil)
	ctx := context.Background()

	tgID := uniqueTgID()
	var userID uuid.UUID
	_ = pool.QueryRow(ctx,
		`INSERT INTO users (telegram_id) VALUES ($1) RETURNING id`, tgID,
	).Scan(&userID)
	_, _ = pool.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID)

	result, err := svc.Lookup(ctx, itoa(tgID))
	if err != nil {
		t.Fatalf("Lookup by tg_id: %v", err)
	}
	if result.User.TelegramID != tgID {
		t.Fatalf("expected tg_id %d, got %d", tgID, result.User.TelegramID)
	}
}

// TestLookup_NotFound: unknown identifier returns ErrAdminUserNotFound.
func TestLookup_NotFound(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, nil)

	_, err := svc.Lookup(context.Background(), "999999999999")
	if err != service.ErrAdminUserNotFound {
		t.Fatalf("expected ErrAdminUserNotFound, got %v", err)
	}
}

// TestLookup_ByKeyPrefix: lookup by "sbf_live_" prefix returns the owner.
func TestLookup_ByKeyPrefix(t *testing.T) {
	pool := newTestPool(t)
	svc := newAdminSvc(t, pool, nil)
	ctx := context.Background()

	userID := insertTestUser(t, pool)
	// Issue a key manually so we know the prefix.
	log, _ := zap.NewDevelopment()
	keySvc := service.NewKeyService(pool, log)
	_, prefix, err := keySvc.Issue(ctx, userID)
	if err != nil {
		t.Fatalf("Issue key: %v", err)
	}

	result, err := svc.Lookup(ctx, prefix)
	if err != nil {
		t.Fatalf("Lookup by key prefix: %v", err)
	}
	if result.User.ID != userID {
		t.Fatalf("expected user %s, got %s", userID, result.User.ID)
	}
}

// ─────────────────────────── Helpers ───────────────────────────────────────

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
