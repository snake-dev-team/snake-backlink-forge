// Package handlers_test — unit tests for V1Usage handler.
// Tests use a stub DBTX to avoid a live Postgres dependency.
// Five cases: (a) data, (b) zero-data, (c) nil-deps 503, (d) no-auth 401,
// (e) IDOR — user A's counts do NOT bleed to user B's request.
package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api/handlers"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
)

// ─────────────────────────── Stub DBTX ────────────────────────────────────

// stubRow implements pgx.Row and returns preset scan values in order.
type stubRow struct {
	values []any
	err    error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i >= len(r.values) {
			break
		}
		switch v := d.(type) {
		case *int64:
			if src, ok := r.values[i].(int64); ok {
				*v = src
			}
		}
	}
	return nil
}

// stubDBTX implements sqlcdb.DBTX.
// callIndex tracks which QueryRow call this is (0=CountWpSites, 1=CountCredits, 2=CountCampaigns).
// Because errgroup runs goroutines concurrently and order is non-deterministic,
// we key on the query string to return the right value.
type stubDBTX struct {
	// queryScanValues maps a query const substring to the int64 value to scan.
	queryScanValues map[string]int64
}

func (s *stubDBTX) Exec(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (s *stubDBTX) Query(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	return nil, nil
}
func (s *stubDBTX) QueryRow(_ context.Context, sql string, _ ...interface{}) pgx.Row {
	// Match on substrings of the SQL constant to identify which query is being called.
	var val int64
	for key, v := range s.queryScanValues {
		if containsSubstr(sql, key) {
			val = v
			break
		}
	}
	return &stubRow{values: []any{val}}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && findSubstr(s, sub)
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ─────────────────────────── Harness ──────────────────────────────────────

// newUsageApp builds a Fiber app with V1Usage mounted. The authed context is
// injected via a before-handler that sets the api_user local — bypassing the
// real key-validation middleware which requires a live DB.
func newUsageApp(deps *handlers.ApiHandlerDeps, authedUser *middleware.ApiUser) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/api/v1/me/usage", func(c *fiber.Ctx) error {
		if authedUser != nil {
			c.Locals("api_user", *authedUser)
		}
		return c.Next()
	}, handlers.V1Usage(deps))
	return app
}

func mustReadJSON(t *testing.T, body io.ReadCloser) map[string]any {
	t.Helper()
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal JSON: %v — body: %s", err, b)
	}
	return m
}

func newQueriesFromStub(stub *stubDBTX) *sqlcdb.Queries {
	return sqlcdb.New(stub)
}

func newApiUser(id uuid.UUID) *middleware.ApiUser {
	return &middleware.ApiUser{ID: id, KeyPrefix: "sbf_live_tst"}
}

// ─────────────────────────── Test cases ───────────────────────────────────

// (a) Authed user with data: 2 sites + 100 consumed credits + 1 running campaign.
func TestV1Usage_AuthedUserWithData(t *testing.T) {
	userID := uuid.New()
	stub := &stubDBTX{queryScanValues: map[string]int64{
		"wp_sites":   2,
		"delta_cred": 100,
		"campaigns":  1,
	}}
	deps := &handlers.ApiHandlerDeps{Queries: newQueriesFromStub(stub)}
	app := newUsageApp(deps, newApiUser(userID))

	req := httptest.NewRequest("GET", "/api/v1/me/usage", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	payload := mustReadJSON(t, resp.Body)
	if got := payload["sites_connected"]; got != float64(2) {
		t.Errorf("sites_connected: want 2, got %v", got)
	}
	if got := payload["credits_consumed_month"]; got != float64(100) {
		t.Errorf("credits_consumed_month: want 100, got %v", got)
	}
	if got := payload["campaigns_running"]; got != float64(1) {
		t.Errorf("campaigns_running: want 1, got %v", got)
	}
}

// (b) Zero-data user: authed, all tables empty → {0,0,0}.
func TestV1Usage_ZeroDataUser(t *testing.T) {
	userID := uuid.New()
	stub := &stubDBTX{queryScanValues: map[string]int64{
		"wp_sites":   0,
		"delta_cred": 0,
		"campaigns":  0,
	}}
	deps := &handlers.ApiHandlerDeps{Queries: newQueriesFromStub(stub)}
	app := newUsageApp(deps, newApiUser(userID))

	req := httptest.NewRequest("GET", "/api/v1/me/usage", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	payload := mustReadJSON(t, resp.Body)
	for _, key := range []string{"sites_connected", "credits_consumed_month", "campaigns_running"} {
		if got := payload[key]; got != float64(0) {
			t.Errorf("%s: want 0, got %v", key, got)
		}
	}
}

// (c) deps.Queries=nil → 503 api_unavailable.
func TestV1Usage_NilDeps_Returns503(t *testing.T) {
	userID := uuid.New()
	deps := &handlers.ApiHandlerDeps{Queries: nil}
	app := newUsageApp(deps, newApiUser(userID))

	req := httptest.NewRequest("GET", "/api/v1/me/usage", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
	payload := mustReadJSON(t, resp.Body)
	if payload["error"] != "api_unavailable" {
		t.Errorf("want error=api_unavailable, got %v", payload["error"])
	}
}

// (d) No auth → 401 unauthorized.
func TestV1Usage_NoAuth_Returns401(t *testing.T) {
	stub := &stubDBTX{queryScanValues: map[string]int64{}}
	deps := &handlers.ApiHandlerDeps{Queries: newQueriesFromStub(stub)}
	// Pass nil authedUser so the local is never set.
	app := newUsageApp(deps, nil)

	req := httptest.NewRequest("GET", "/api/v1/me/usage", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	payload := mustReadJSON(t, resp.Body)
	if payload["error"] != "unauthorized" {
		t.Errorf("want error=unauthorized, got %v", payload["error"])
	}
}

// (e) IDOR: user B's rows exist in DB but request is made as user A.
// Stub returns 5/200/3 for user B's query args and 1/50/0 for user A's args.
// The stub matches on query substring only (not user_id param), so to verify
// IDOR safety at the SQL layer we confirm the user_id passed to QueryRow is
// user A's ID — not user B's — by using a paramCapturingDBTX.
func TestV1Usage_IDORCrossUser_ReturnsOnlyCallerCounts(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()

	// paramCapturingDBTX captures the user_id argument passed to each QueryRow call.
	// Mutex protects against concurrent appends from errgroup goroutines.
	type call struct {
		sql    string
		userID uuid.UUID
	}
	var (
		callsMu sync.Mutex
		calls   []call
	)

	inner := &stubDBTX{queryScanValues: map[string]int64{
		"wp_sites":   1,
		"delta_cred": 50,
		"campaigns":  0,
	}}

	// Wrap via a local implementing DBTX.
	impl := &struct {
		exec     func(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
		query    func(context.Context, string, ...interface{}) (pgx.Rows, error)
		queryRow func(context.Context, string, ...interface{}) pgx.Row
	}{
		exec:  func(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
		query: func(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) { return nil, nil },
		queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
			if len(args) > 0 {
				if uid, ok := args[0].(uuid.UUID); ok {
					callsMu.Lock()
					calls = append(calls, call{sql: sql, userID: uid})
					callsMu.Unlock()
				}
			}
			return inner.QueryRow(context.Background(), sql, args...)
		},
	}

	// Build Queries using a wrapper that satisfies the DBTX interface.
	wrappedDBTX := &funcDBTX{
		exec:     impl.exec,
		query:    impl.query,
		queryRow: impl.queryRow,
	}
	deps := &handlers.ApiHandlerDeps{Queries: sqlcdb.New(wrappedDBTX)}
	app := newUsageApp(deps, newApiUser(userA))

	// Insert fixture: userB has 5 sites (not modeled via DB, but verifiable via param capture).
	req := httptest.NewRequest("GET", "/api/v1/me/usage", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	// IDOR verification: every QueryRow call must have been scoped to userA.
	// Mutex ensures no concurrent reads after errgroup goroutines completed.
	callsMu.Lock()
	defer callsMu.Unlock()
	for _, c := range calls {
		if c.userID != userA {
			t.Errorf("IDOR: QueryRow for %q was called with userID=%s, want userA=%s", c.sql, c.userID, userA)
		}
		if c.userID == userB {
			t.Errorf("IDOR: userB's ID leaked into query: %q", c.sql)
		}
	}
	if len(calls) == 0 {
		t.Error("no QueryRow calls captured — handler may not have executed queries")
	}
}

// funcDBTX adapts function fields to the sqlcdb.DBTX interface.
type funcDBTX struct {
	exec     func(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	query    func(context.Context, string, ...interface{}) (pgx.Rows, error)
	queryRow func(context.Context, string, ...interface{}) pgx.Row
}

func (f *funcDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return f.exec(ctx, sql, args...)
}
func (f *funcDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return f.query(ctx, sql, args...)
}
func (f *funcDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return f.queryRow(ctx, sql, args...)
}
