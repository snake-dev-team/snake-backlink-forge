// admin_alerts_watcher_test.go — Phase 08: integration tests for AuditFailAlertWatcher.
// Requires DATABASE_URL env var (live Postgres). Uses package bot (white-box: calls
// auditFailAlertWatcherWithTicker directly for fast tick injection).
package bot

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"go.uber.org/zap"
)

// newWatcherTestPool opens a pgxpool from DATABASE_URL or skips.
func newWatcherTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../.env")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping watcher integration tests")
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

// seedAuthFailRows inserts n audit_log rows with event='sepay_auth_fail' within the last 15 min.
// Returns inserted IDs for cleanup.
func seedAuthFailRows(t *testing.T, pool *pgxpool.Pool, n int) []int64 {
	t.Helper()
	ctx := context.Background()
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO audit_log (event, metadata, created_at)
			 VALUES ('sepay_auth_fail', '{}', NOW() - INTERVAL '1 minute')
			 RETURNING id`,
		).Scan(&id)
		if err != nil {
			t.Fatalf("seedAuthFailRows[%d]: %v", i, err)
		}
		ids = append(ids, id)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = pool.Exec(context.Background(),
				`DELETE FROM audit_log WHERE id = $1`, id)
		}
	})
	return ids
}

// TestAuditFailAlertWatcher_BurstFiresAlert: seed 20 auth_fail rows → watcher fires alert within 2s.
func TestAuditFailAlertWatcher_BurstFiresAlert(t *testing.T) {
	pool := newWatcherTestPool(t)
	seedAuthFailRows(t, pool, 20)

	alertCh := make(chan notify.AdminAlert, 5)
	log, _ := zap.NewDevelopment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use 500ms tick for fast test execution.
	go auditFailAlertWatcherWithTicker(ctx, pool, alertCh, log, 500*time.Millisecond)

	select {
	case alert := <-alertCh:
		if alert.Kind != "auth_fail_burst" {
			t.Fatalf("expected auth_fail_burst alert, got %q", alert.Kind)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not fire alert within 3s")
	}
}

// TestAuditFailAlertWatcher_Dedup: same burst → alert fires once, second tick does NOT send duplicate.
func TestAuditFailAlertWatcher_Dedup(t *testing.T) {
	pool := newWatcherTestPool(t)
	seedAuthFailRows(t, pool, 20)

	alertCh := make(chan notify.AdminAlert, 5)
	log, _ := zap.NewDevelopment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use 500ms tick — two ticks will fire within the same 15-min bucket.
	go auditFailAlertWatcherWithTicker(ctx, pool, alertCh, log, 500*time.Millisecond)

	// Wait for first alert.
	select {
	case <-alertCh:
		// first alert received — expected
	case <-time.After(3 * time.Second):
		t.Fatal("first alert did not fire within 3s")
	}

	// Wait 1.5 seconds (3 more ticks) — no duplicate alert should arrive.
	select {
	case extra := <-alertCh:
		t.Fatalf("duplicate alert fired within same bucket: %+v", extra)
	case <-time.After(1500 * time.Millisecond):
		// Correct: no duplicate
	}
}
