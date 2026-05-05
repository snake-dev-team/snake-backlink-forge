// campaign_service_test.go — integration tests for CampaignService using live Postgres.
// Requires DATABASE_URL env var. Each test uses unique users; tables NOT truncated.
//
// Covered invariants:
//   [F12] IDOR guard: List/Get return only caller's own campaigns
//   [F13] SSRF guard: money_site_url rejects private/loopback/metadata IPs
//   Create / List / Get / SetStatus happy + validation paths
//
// Shared helpers (randSuffix, insertTestUserWithCredits, etc.) live in
// service_test_harness_test.go (same package service_test).
package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
)

// ─────────────────────────── Create tests ─────────────────────────────────

func TestCampaignServiceCreate_Valid(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)

	c, err := svc.Create(context.Background(), userID, validCampaignInput("standard"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Status != sqlcdb.CampaignStatusDraft {
		t.Fatalf("expected status=draft, got %s", c.Status)
	}
	if c.UserID != userID {
		t.Fatalf("user_id mismatch: want %s, got %s", userID, c.UserID)
	}
}

func TestCampaignServiceCreate_StartNow(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)

	in := validCampaignInput("standard")
	in.StartNow = true
	c, err := svc.Create(context.Background(), userID, in)
	if err != nil {
		t.Fatalf("Create StartNow: %v", err)
	}
	if c.Status != sqlcdb.CampaignStatusRunning {
		t.Fatalf("expected status=running, got %s", c.Status)
	}
}

func TestCampaignServiceCreate_InvalidName(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)
	ctx := context.Background()

	cases := []struct{ name, value string }{
		{"too short", "ab"},
		{"too long", strings.Repeat("x", 129)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			in := validCampaignInput("standard")
			in.Name = tc.value
			_, err := svc.Create(ctx, userID, in)
			if err == nil {
				t.Fatalf("expected error for name=%q, got nil", tc.value)
			}
		})
	}
}

func TestCampaignServiceCreate_InvalidURL(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)
	ctx := context.Background()

	cases := []string{"", "example.com", "javascript:alert(1)", "ftp://example.com"}
	for _, url := range cases {
		url := url
		t.Run(url, func(t *testing.T) {
			in := validCampaignInput("standard")
			in.MoneySiteURL = url
			_, err := svc.Create(ctx, userID, in)
			if err == nil {
				t.Fatalf("expected ErrCampaignInvalid for url=%q, got nil", url)
			}
		})
	}
}

// TestCampaignServiceCreate_SSRFGuard verifies F13: private/loopback/metadata IPs rejected.
func TestCampaignServiceCreate_SSRFGuard(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)
	ctx := context.Background()

	ssrfURLs := []string{
		"http://localhost",
		"http://127.0.0.1",
		"http://169.254.169.254",
		"http://10.0.0.1",
		"http://192.168.1.1",
	}
	for _, u := range ssrfURLs {
		u := u
		t.Run(u, func(t *testing.T) {
			in := validCampaignInput("standard")
			in.MoneySiteURL = u
			_, err := svc.Create(ctx, userID, in)
			if err == nil {
				t.Fatalf("F13: expected ErrCampaignInvalid for SSRF url=%q, got nil", u)
			}
		})
	}
}

func TestCampaignServiceCreate_InvalidPool(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)

	in := validCampaignInput("gold") // invalid pool
	_, err := svc.Create(context.Background(), userID, in)
	if err == nil {
		t.Fatal("expected ErrCampaignInvalid for pool=gold, got nil")
	}
}

func TestCampaignServiceCreate_InvalidSourceMode(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)

	in := validCampaignInput("standard")
	in.SourceMode = "unknown"
	_, err := svc.Create(context.Background(), userID, in)
	if err == nil {
		t.Fatal("expected ErrCampaignInvalid for source_mode=unknown, got nil")
	}
}

func TestCampaignServiceCreate_DailyLimitBounds(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)
	ctx := context.Background()

	invalid := []int32{4, 51}
	for _, limit := range invalid {
		in := validCampaignInput("standard")
		in.DailyLimit = limit
		_, err := svc.Create(ctx, userID, in)
		if err == nil {
			t.Fatalf("expected error for daily_limit=%d, got nil", limit)
		}
	}
	valid := []int32{5, 50}
	for _, limit := range valid {
		in := validCampaignInput("standard")
		in.DailyLimit = limit
		_, err := svc.Create(ctx, userID, in)
		if err != nil {
			t.Fatalf("unexpected error for daily_limit=%d: %v", limit, err)
		}
	}
}

func TestCampaignServiceCreate_AnchorBounds(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)
	ctx := context.Background()

	// 0 anchors → error.
	in0 := validCampaignInput("standard")
	in0.AnchorTexts = nil
	if _, err := svc.Create(ctx, userID, in0); err == nil {
		t.Fatal("expected error for 0 anchors, got nil")
	}

	// 21 anchors → error.
	in21 := validCampaignInput("standard")
	anchors := make([]service.AnchorTextInput, 21)
	for i := range anchors {
		anchors[i] = service.AnchorTextInput{Text: "anchor", Type: "branded", Weight: 1}
	}
	in21.AnchorTexts = anchors
	if _, err := svc.Create(ctx, userID, in21); err == nil {
		t.Fatal("expected error for 21 anchors, got nil")
	}
}

func TestCampaignServiceCreate_AnchorWeight(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)
	ctx := context.Background()

	for _, w := range []int32{0, 101} {
		in := validCampaignInput("standard")
		in.AnchorTexts = []service.AnchorTextInput{{Text: "anchor", Type: "branded", Weight: w}}
		_, err := svc.Create(ctx, userID, in)
		if err == nil {
			t.Fatalf("expected error for anchor weight=%d, got nil", w)
		}
	}
}

func TestCampaignServiceCreate_AnchorType(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)
	ctx := context.Background()

	// Invalid type → error.
	in := validCampaignInput("standard")
	in.AnchorTexts = []service.AnchorTextInput{{Text: "anchor", Type: "unknown", Weight: 1}}
	if _, err := svc.Create(ctx, userID, in); err == nil {
		t.Fatal("expected error for anchor type=unknown, got nil")
	}

	// Valid types → OK.
	for _, tp := range []string{"branded", "naked", "generic", "exact"} {
		tp := tp
		t.Run(tp, func(t *testing.T) {
			in := validCampaignInput("standard")
			in.AnchorTexts = []service.AnchorTextInput{{Text: "anchor", Type: tp, Weight: 1}}
			if _, err := svc.Create(ctx, userID, in); err != nil {
				t.Fatalf("unexpected error for anchor type=%s: %v", tp, err)
			}
		})
	}
}

func TestCampaignServiceCreate_NicheCompact(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 100, 100)

	in := validCampaignInput("standard")
	// Duplicates + 25 entries (exceeds max 20).
	in.NicheKeywords = make([]string, 25)
	for i := range in.NicheKeywords {
		if i%5 == 0 {
			in.NicheKeywords[i] = "dup"
		} else {
			in.NicheKeywords[i] = "kw" + randSuffix()
		}
	}
	c, err := svc.Create(context.Background(), userID, in)
	if err != nil {
		t.Fatalf("Create NicheCompact: %v", err)
	}
	// After compactStrings(max=20): ≤20 entries, no duplicates.
	if len(c.NicheKeywords) > 20 {
		t.Fatalf("niche_keywords exceeded 20: got %d", len(c.NicheKeywords))
	}
	seen := map[string]bool{}
	for _, kw := range c.NicheKeywords {
		if seen[kw] {
			t.Fatalf("duplicate keyword in result: %q", kw)
		}
		seen[kw] = true
	}
}

// ─────────────────────────── List tests ───────────────────────────────────

func TestCampaignServiceList_Pagination(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	// Insert 15 campaigns.
	for i := 0; i < 15; i++ {
		if _, err := svc.Create(ctx, userID, validCampaignInput("standard")); err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
	}

	page1, err := svc.List(ctx, userID, 10, 0)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1) != 10 {
		t.Fatalf("page1: want 10, got %d", len(page1))
	}

	page2, err := svc.List(ctx, userID, 10, 10)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2) != 5 {
		t.Fatalf("page2: want 5, got %d", len(page2))
	}
}

func TestCampaignServiceList_LimitBounds(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	// No campaigns needed — just verify default limit doesn't error.
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	// limit<=0 and limit>100 should default to 25 (no panic/error).
	for _, limit := range []int32{0, -1, 101, 200} {
		rows, err := svc.List(ctx, userID, limit, 0)
		if err != nil {
			t.Fatalf("List(limit=%d): unexpected error: %v", limit, err)
		}
		_ = rows // empty for this user; we just verify no crash
	}
}

// TestCampaignServiceList_UserScoped verifies F12: userA's List never returns userB's campaigns.
func TestCampaignServiceList_UserScoped(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userA := insertTestUserWithCredits(t, pool, 0, 0)
	userB := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	// userA: 5 campaigns.
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, userA, validCampaignInput("standard")); err != nil {
			t.Fatalf("userA Create[%d]: %v", i, err)
		}
	}
	// userB: 3 campaigns.
	for i := 0; i < 3; i++ {
		if _, err := svc.Create(ctx, userB, validCampaignInput("standard")); err != nil {
			t.Fatalf("userB Create[%d]: %v", i, err)
		}
	}

	rows, err := svc.List(ctx, userA, 100, 0)
	if err != nil {
		t.Fatalf("userA List: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("F12: userA List returned %d rows, want 5", len(rows))
	}
	for _, r := range rows {
		if r.UserID != userA {
			t.Fatalf("F12 IDOR: found userB's campaign in userA's list: %s", r.ID)
		}
	}
}

// ─────────────────────────── Get tests ────────────────────────────────────

func TestCampaignServiceGet_Found(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	created, err := svc.Create(ctx, userID, validCampaignInput("standard"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := svc.Get(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("Get returned wrong campaign: %s vs %s", got.ID, created.ID)
	}
}

func TestCampaignServiceGet_NotFound(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	_, err := svc.Get(context.Background(), userID, uuid.New())
	if err == nil {
		t.Fatal("expected ErrCampaignNotFound for random UUID, got nil")
	}
}

// TestCampaignServiceGet_CrossUser verifies F12: userB cannot read userA's campaign.
func TestCampaignServiceGet_CrossUser(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userA := insertTestUserWithCredits(t, pool, 0, 0)
	userB := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	c, err := svc.Create(ctx, userA, validCampaignInput("standard"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = svc.Get(ctx, userB, c.ID) // userB tries to read userA's campaign
	if err == nil {
		t.Fatal("F12 IDOR: expected ErrCampaignNotFound when cross-user Get, got nil")
	}
}

// ─────────────────────────── SetStatus tests ──────────────────────────────

func TestCampaignServiceSetStatus_ValidTransition(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	c, err := svc.Create(ctx, userID, validCampaignInput("standard"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// draft → running
	c, err = svc.SetStatus(ctx, userID, c.ID, sqlcdb.CampaignStatusRunning)
	if err != nil {
		t.Fatalf("SetStatus running: %v", err)
	}
	if c.Status != sqlcdb.CampaignStatusRunning {
		t.Fatalf("expected running, got %s", c.Status)
	}

	// running → paused
	c, err = svc.SetStatus(ctx, userID, c.ID, sqlcdb.CampaignStatusPaused)
	if err != nil {
		t.Fatalf("SetStatus paused: %v", err)
	}
	if c.Status != sqlcdb.CampaignStatusPaused {
		t.Fatalf("expected paused, got %s", c.Status)
	}

	// paused → archived
	c, err = svc.SetStatus(ctx, userID, c.ID, sqlcdb.CampaignStatusArchived)
	if err != nil {
		t.Fatalf("SetStatus archived: %v", err)
	}
	if c.Status != sqlcdb.CampaignStatusArchived {
		t.Fatalf("expected archived, got %s", c.Status)
	}
}

func TestCampaignServiceSetStatus_InvalidStatus(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	c, err := svc.Create(ctx, userID, validCampaignInput("standard"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// "draft" is not in the allowed set for SetStatus (running/paused/completed/archived only).
	_, err = svc.SetStatus(ctx, userID, c.ID, sqlcdb.CampaignStatusDraft)
	if err == nil {
		t.Fatal("expected ErrCampaignInvalid for status=draft in SetStatus, got nil")
	}
}

func TestCampaignServiceSetStatus_NotFound(t *testing.T) {
	pool := newTestPool(t)
	svc := setupCampaignService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	_, err := svc.SetStatus(context.Background(), userID, uuid.New(), sqlcdb.CampaignStatusRunning)
	if err == nil {
		t.Fatal("expected ErrCampaignNotFound for random UUID, got nil")
	}
}
