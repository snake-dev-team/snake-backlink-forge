// audit_service_test.go — integration tests for AuditService using live Postgres.
// Requires DATABASE_URL env var. Uses service_test package (same binary as other service tests).
package service_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

func newAuditSvc(t *testing.T, pool *pgxpool.Pool) *service.AuditService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	return service.NewAuditService(pool, q, log.Named("audit_test"))
}

// TestLog_Insert verifies a single audit row is inserted and returns a positive ID.
func TestLog_Insert(t *testing.T) {
	pool := newTestPool(t)
	svc := newAuditSvc(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	id, err := svc.Log(ctx, service.AuditInput{
		UserID:   &userID,
		Event:    "test_event",
		Metadata: map[string]any{"test": true},
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive audit ID, got %d", id)
	}
}

// TestLog_NilUser verifies a system event with no user and no key inserts cleanly.
// audit_log.user_id and key_id are both nullable; event name carries the identity.
func TestLog_NilUser(t *testing.T) {
	pool := newTestPool(t)
	svc := newAuditSvc(t, pool)
	ctx := context.Background()

	// Both UserID and KeyID are nil — system-level event (e.g. startup health check).
	id, err := svc.Log(ctx, service.AuditInput{
		UserID: nil,
		KeyID:  nil,
		Event:  "system_event",
	})
	if err != nil {
		t.Fatalf("Log with nil user+key: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive audit ID for nil-user event, got %d", id)
	}
}

// TestLogIntoTx_Rollback verifies that rolling back the tx leaves NO audit row inserted.
func TestLogIntoTx_Rollback(t *testing.T) {
	pool := newTestPool(t)
	svc := newAuditSvc(t, pool)
	ctx := context.Background()

	userID := insertTestUser(t, pool)

	var beforeCount int64
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1`, userID,
	).Scan(&beforeCount); err != nil {
		t.Fatalf("count before: %v", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	_, logErr := svc.LogIntoTx(ctx, tx, service.AuditInput{
		UserID: &userID,
		Event:  "test_rollback_event",
	})
	// Always rollback in this test — verify audit row disappears.
	_ = tx.Rollback(ctx)

	if logErr != nil {
		t.Fatalf("LogIntoTx: %v", logErr)
	}

	var afterCount int64
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1`, userID,
	).Scan(&afterCount); err != nil {
		t.Fatalf("count after: %v", err)
	}

	if afterCount != beforeCount {
		t.Fatalf("audit row present after rollback: before=%d after=%d", beforeCount, afterCount)
	}
}
