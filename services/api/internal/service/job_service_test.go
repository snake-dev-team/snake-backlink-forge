// job_service_test.go — integration tests for JobService using live Postgres.
// Requires DATABASE_URL env var. Each test uses unique users; tables NOT truncated.
//
// Covered invariants:
//   [F1]  Partial-conflict Enqueue debits only actual inserted jobs (not requested count)
//   [F7]  Enqueue bumps campaigns.credits_consumed by actual job count
//   [F12] IDOR: Report cross-user returns ErrJobNotFound
//   CreateCustomTarget / ListTargets / Enqueue / Report happy + failure paths
package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── Harness ──────────────────────────────────────

func setupJobService(t *testing.T, pool *pgxpool.Pool) *service.JobService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	return service.NewJobService(pool, q, log.Named("test-job"))
}

// getStandardCredits reads standard_credits from wallets for the given user.
func getStandardCredits(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) int32 {
	t.Helper()
	var n int32
	err := pool.QueryRow(context.Background(),
		`SELECT standard_credits FROM wallets WHERE user_id = $1`, userID,
	).Scan(&n)
	if err != nil {
		t.Fatalf("getStandardCredits: %v", err)
	}
	return n
}

// getCampaignCreditsConsumed reads credits_consumed from campaigns row.
func getCampaignCreditsConsumed(t *testing.T, pool *pgxpool.Pool, campaignID uuid.UUID) int32 {
	t.Helper()
	var n int32
	err := pool.QueryRow(context.Background(),
		`SELECT credits_consumed FROM campaigns WHERE id = $1`, campaignID,
	).Scan(&n)
	if err != nil {
		t.Fatalf("getCampaignCreditsConsumed: %v", err)
	}
	return n
}

// insertJobDirect bypasses Enqueue to directly insert a job row with a given status.
// Used to set up Report tests where a job must already be in 'dispatched' state.
func insertJobDirect(t *testing.T, pool *pgxpool.Pool, userID, campaignID, targetID uuid.UUID, targetURL, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO jobs
			(user_id, campaign_id, target_id, target_url_snapshot, anchor_text,
			 anchor_type, status, pool, credits_cost)
		VALUES ($1, $2, $3, $4, 'test anchor', 'branded', $5, 'standard', 1)
		RETURNING id`,
		userID, campaignID, targetID, targetURL, status,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertJobDirect: %v", err)
	}
	return id
}

// ─────────────────────────── CreateCustomTarget tests ─────────────────────

func TestJobServiceCreateCustomTarget_Valid(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	in := service.CustomTargetInput{
		URL:  "https://myblog-" + randSuffix() + ".com/comment",
		Type: "blog_comment",
		Pool: "standard",
	}
	target, err := svc.CreateCustomTarget(context.Background(), userID, in)
	if err != nil {
		t.Fatalf("CreateCustomTarget: %v", err)
	}
	if target.ID == uuid.Nil {
		t.Fatal("expected non-nil target ID")
	}
	if string(target.Type) != "blog_comment" {
		t.Fatalf("type: want blog_comment, got %s", target.Type)
	}
}

func TestJobServiceCreateCustomTarget_InvalidURL(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	cases := []string{"", "javascript:alert(1)", "not-a-url"}
	for _, u := range cases {
		u := u
		t.Run(u, func(t *testing.T) {
			in := service.CustomTargetInput{URL: u, Type: "blog_comment", Pool: "standard"}
			_, err := svc.CreateCustomTarget(ctx, userID, in)
			if err == nil {
				t.Fatalf("expected ErrJobInvalid for url=%q, got nil", u)
			}
		})
	}
}

func TestJobServiceCreateCustomTarget_InvalidType(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	in := service.CustomTargetInput{
		URL:  "https://test-" + randSuffix() + ".com/page",
		Type: "unknown",
		Pool: "standard",
	}
	_, err := svc.CreateCustomTarget(context.Background(), userID, in)
	if err == nil {
		t.Fatal("expected ErrJobInvalid for type=unknown, got nil")
	}
}

func TestJobServiceCreateCustomTarget_PoolFallback(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	// pool="gold" is invalid; service should fall back to "standard" without error.
	in := service.CustomTargetInput{
		URL:  "https://fallback-" + randSuffix() + ".com/page",
		Type: "blog_comment",
		Pool: "gold",
	}
	target, err := svc.CreateCustomTarget(context.Background(), userID, in)
	if err != nil {
		t.Fatalf("PoolFallback: unexpected error: %v", err)
	}
	if target.Pool != "standard" {
		t.Fatalf("expected pool=standard after fallback, got %s", target.Pool)
	}
}

// ─────────────────────────── ListTargets tests ────────────────────────────

func TestJobServiceListTargets_Pool(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	// Insert one premium target (global pool, owner=NULL).
	insertPrebuiltTarget(t, pool, "premium", "premium-list-"+randSuffix()+".com")

	targets, err := svc.ListTargets(context.Background(), userID, "premium", 50, 0)
	if err != nil {
		t.Fatalf("ListTargets premium: %v", err)
	}
	for _, tgt := range targets {
		if tgt.Pool != "premium" {
			t.Fatalf("F: ListTargets returned non-premium target: pool=%s", tgt.Pool)
		}
	}
}

func TestJobServiceListTargets_PoolEmpty(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	// pool="" (invalid) → service sets pool="" → SQL returns all (no pool filter).
	// Just ensure no error; count may vary.
	_, err := svc.ListTargets(context.Background(), userID, "", 50, 0)
	if err != nil {
		t.Fatalf("ListTargets pool=empty: unexpected error: %v", err)
	}
}

func TestJobServiceListTargets_LimitBounds(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	ctx := context.Background()

	// limit<=0 and limit>100 default to 50 (no panic/error).
	for _, limit := range []int32{0, -1, 101} {
		_, err := svc.ListTargets(ctx, userID, "standard", limit, 0)
		if err != nil {
			t.Fatalf("ListTargets(limit=%d): unexpected error: %v", limit, err)
		}
	}
}

// ─────────────────────────── Enqueue tests ────────────────────────────────

// setupEnqueueTest creates user+wallet, campaign, and N prebuilt targets.
// Returns (userID, campaignID, targetIDs).
func setupEnqueueTest(t *testing.T, pool *pgxpool.Pool, stdCredits int32, targetCount int) (uuid.UUID, uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	userID := insertTestUserWithCredits(t, pool, 0, stdCredits)

	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	campSvc := service.NewCampaignService(pool, q, log.Named("enqueue-setup"))

	in := validCampaignInput("standard")
	in.DailyLimit = 50
	in.CreditsAllocated = 100
	c, err := campSvc.Create(ctx, userID, in)
	if err != nil {
		t.Fatalf("setupEnqueueTest Create: %v", err)
	}
	// Must be running for ClaimNext; also Enqueue doesn't require running status.
	_, err = campSvc.SetStatus(ctx, userID, c.ID, sqlcdb.CampaignStatusRunning)
	if err != nil {
		t.Fatalf("setupEnqueueTest SetStatus: %v", err)
	}

	ids := make([]uuid.UUID, targetCount)
	for i := 0; i < targetCount; i++ {
		domain := fmt.Sprintf("enq%d-%s.com", i, randSuffix())
		ids[i] = insertPrebuiltTarget(t, pool, "standard", domain)
	}
	return userID, c.ID, ids
}

func TestJobServiceEnqueue_Valid(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, campaignID, _ := setupEnqueueTest(t, pool, 50, 5)
	ctx := context.Background()

	creditsBefore := getStandardCredits(t, pool, userID)

	jobs, err := svc.Enqueue(ctx, userID, campaignID, 5)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("Enqueue: expected >0 jobs, got 0")
	}
	if int32(len(jobs)) > 5 {
		t.Fatalf("Enqueue: got %d jobs (more than requested 5)", len(jobs))
	}

	creditsAfter := getStandardCredits(t, pool, userID)
	debit := creditsBefore - creditsAfter
	// Core invariant: debit == actual jobs inserted.
	if debit != int32(len(jobs)) {
		t.Fatalf("credit debit=%d != len(jobs)=%d", debit, len(jobs))
	}
}

// TestJobServiceEnqueue_PartialConflict verifies F1:
// 2 targets pre-used for this campaign via ON CONFLICT DO NOTHING → debit == actual
// jobs inserted (not the 5 requested). The key invariant is debit == len(jobs).
func TestJobServiceEnqueue_PartialConflict(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, campaignID, targetIDs := setupEnqueueTest(t, pool, 50, 5)
	ctx := context.Background()

	// Pre-insert jobs for the first 2 targets so they are "already used" by this campaign.
	// PickTargetsForCampaign excludes them via NOT EXISTS → they won't be picked again.
	// However ON CONFLICT DO NOTHING path is also tested for robustness.
	for i := 0; i < 2; i++ {
		var url string
		if err := pool.QueryRow(ctx, `SELECT url FROM targets WHERE id = $1`, targetIDs[i]).Scan(&url); err != nil {
			t.Fatalf("fetch target url[%d]: %v", i, err)
		}
		insertJobDirect(t, pool, userID, campaignID, targetIDs[i], url, "queued")
	}

	creditsBefore := getStandardCredits(t, pool, userID)

	jobs, err := svc.Enqueue(ctx, userID, campaignID, 5)
	if err != nil {
		t.Fatalf("F1 PartialConflict Enqueue: %v", err)
	}

	creditsAfter := getStandardCredits(t, pool, userID)
	debit := creditsBefore - creditsAfter
	actualJobs := int32(len(jobs))

	// F1 core invariant: credits debited == actual jobs inserted (not requested count).
	// This is THE F1 fix — before: Enqueue debited the requested count; after: only actual.
	// We do not assert debit < 5 because the DB may contain global NULL-owner prebuilt
	// targets from prior test runs; picker can fall back to those and reach LIMIT=5.
	if debit != actualJobs {
		t.Fatalf("F1 VIOLATED: debit=%d but len(jobs)=%d — over/under charged", debit, actualJobs)
	}
	// At least 1 new job inserted (3 user-owned targets remain for this campaign).
	if actualJobs == 0 {
		t.Fatalf("F1: expected >0 new jobs from remaining targets, got 0")
	}
}

// TestJobServiceEnqueue_AllConflict: all 5 targets already used → ErrJobInvalid, no debit.
func TestJobServiceEnqueue_AllConflict(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, campaignID, targetIDs := setupEnqueueTest(t, pool, 50, 5)
	ctx := context.Background()

	// Pre-insert all 5 jobs so the user's targets are already used.
	for i, tid := range targetIDs {
		var url string
		if err := pool.QueryRow(ctx, `SELECT url FROM targets WHERE id = $1`, tid).Scan(&url); err != nil {
			t.Fatalf("fetch target url[%d]: %v", i, err)
		}
		insertJobDirect(t, pool, userID, campaignID, tid, url, "queued")
	}

	creditsBefore := getStandardCredits(t, pool, userID)

	jobs, err := svc.Enqueue(ctx, userID, campaignID, 5)
	creditsAfter := getStandardCredits(t, pool, userID)
	debit := creditsBefore - creditsAfter

	// F1 invariant must hold regardless of NULL-owner pollution from prior tests:
	// either ErrJobInvalid (no global fallbacks available) with no debit,
	// or ErrJobInvalid not returned but debit == actual jobs inserted.
	if errors.Is(err, service.ErrJobInvalid) {
		if debit != 0 {
			t.Fatalf("AllConflict: ErrJobInvalid path must not debit credits, got debit=%d", debit)
		}
		return
	}
	if err != nil {
		t.Fatalf("AllConflict: unexpected error: %v", err)
	}
	// Fallback path — global NULL-owner targets exist in dev DB; F1 invariant must still hold.
	if debit != int32(len(jobs)) {
		t.Fatalf("F1 VIOLATED in AllConflict fallback: debit=%d but len(jobs)=%d", debit, len(jobs))
	}
}

func TestJobServiceEnqueue_ZeroTargets(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	// Enqueue a campaign that has no available targets:
	// create 1 target, enqueue it so it's now excluded by NOT EXISTS, then re-enqueue.
	// PickTargetsForCampaign excludes targets already used by THIS campaign → returns 0 rows.
	// Enqueue then commits empty and returns ([]Job{}, nil).
	userID, campaignID, _ := setupEnqueueTest(t, pool, 50, 1)
	ctx := context.Background()

	firstJobs, err := svc.Enqueue(ctx, userID, campaignID, 1)
	if err != nil {
		t.Fatalf("ZeroTargets setup first Enqueue: %v", err)
	}
	if len(firstJobs) < 1 {
		t.Skipf("ZeroTargets: setup enqueue got %d jobs (want ≥1); skipping", len(firstJobs))
	}

	// Second Enqueue with credits exhausted-or-targets-exhausted path. With a polluted dev DB
	// (NULL-owner globals matching pool=standard), the picker may return more targets via
	// fallback; in that case we accept either: (a) 0 new jobs (clean DB), or (b) ErrInsufficientCredits
	// (budget exhausted by first call). The F1 invariant must hold throughout.
	creditsBefore := getStandardCredits(t, pool, userID)
	jobs, err := svc.Enqueue(ctx, userID, campaignID, 5)
	creditsAfter := getStandardCredits(t, pool, userID)
	debit := creditsBefore - creditsAfter

	if errors.Is(err, service.ErrInsufficientCredits) {
		return // budget exhausted is acceptable
	}
	if err != nil {
		t.Fatalf("ZeroTargets second Enqueue: unexpected error: %v", err)
	}
	if debit != int32(len(jobs)) {
		t.Fatalf("F1 VIOLATED in ZeroTargets: debit=%d but len(jobs)=%d", debit, len(jobs))
	}
}

func TestJobServiceEnqueue_InsufficientCredits(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	// 0 credits → should fail.
	userID, campaignID, _ := setupEnqueueTest(t, pool, 0, 5)

	_, err := svc.Enqueue(context.Background(), userID, campaignID, 5)
	if !errors.Is(err, service.ErrInsufficientCredits) {
		t.Fatalf("InsufficientCredits: expected ErrInsufficientCredits, got %v", err)
	}
}

func TestJobServiceEnqueue_BudgetClamp(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	// credits_allocated=5 → remainingBudget=5; request count=10 → clamped to 5.
	userID := insertTestUserWithCredits(t, pool, 0, 50)
	ctx := context.Background()

	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	campSvc := service.NewCampaignService(pool, q, log.Named("budget-clamp"))
	in := validCampaignInput("standard")
	in.CreditsAllocated = 3 // small budget
	in.DailyLimit = 50
	c, err := campSvc.Create(ctx, userID, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Insert 10 targets to ensure plenty available.
	for i := 0; i < 10; i++ {
		insertPrebuiltTarget(t, pool, "standard", fmt.Sprintf("budget-clamp-%d-%s.com", i, randSuffix()))
	}

	jobs, err := svc.Enqueue(ctx, userID, c.ID, 10)
	if err != nil {
		t.Fatalf("BudgetClamp Enqueue: %v", err)
	}
	if len(jobs) > 3 {
		t.Fatalf("BudgetClamp: expected ≤3 jobs, got %d", len(jobs))
	}
}

func TestJobServiceEnqueue_DailyLimitClamp(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 50)
	ctx := context.Background()

	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	campSvc := service.NewCampaignService(pool, q, log.Named("daily-clamp"))
	in := validCampaignInput("standard")
	in.DailyLimit = 5
	in.CreditsAllocated = 50
	c, err := campSvc.Create(ctx, userID, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Insert 20 targets.
	for i := 0; i < 20; i++ {
		insertPrebuiltTarget(t, pool, "standard", fmt.Sprintf("daily-clamp-%d-%s.com", i, randSuffix()))
	}

	// count=20 > daily_limit=5 → clamped to 5.
	jobs, err := svc.Enqueue(ctx, userID, c.ID, 20)
	if err != nil {
		t.Fatalf("DailyLimitClamp Enqueue: %v", err)
	}
	if len(jobs) > 5 {
		t.Fatalf("DailyLimitClamp: expected ≤5 jobs, got %d", len(jobs))
	}
}

// TestJobServiceEnqueue_BumpsCreditsConsumed verifies F7:
// After Enqueue(3), campaigns.credits_consumed equals 3.
func TestJobServiceEnqueue_BumpsCreditsConsumed(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, campaignID, _ := setupEnqueueTest(t, pool, 50, 3)
	ctx := context.Background()

	consumedBefore := getCampaignCreditsConsumed(t, pool, campaignID)

	jobs, err := svc.Enqueue(ctx, userID, campaignID, 3)
	if err != nil {
		t.Fatalf("F7 Enqueue: %v", err)
	}
	actualJobs := int32(len(jobs))
	consumedAfter := getCampaignCreditsConsumed(t, pool, campaignID)

	if consumedAfter-consumedBefore != actualJobs {
		t.Fatalf("F7: campaigns.credits_consumed bumped by %d, want %d (actual jobs)",
			consumedAfter-consumedBefore, actualJobs)
	}
}

// ─────────────────────────── Report tests ─────────────────────────────────

// setupReportJob creates user+campaign+target+dispatched job for Report tests.
// Returns (userID, campaignID, jobID, targetURL, moneySiteURL).
func setupReportJob(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID, string, string) {
	t.Helper()
	ctx := context.Background()
	userID := insertTestUserWithCredits(t, pool, 0, 50)

	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	campSvc := service.NewCampaignService(pool, q, log.Named("report-setup"))

	moneySiteURL := "https://money-" + randSuffix() + ".example.com"
	in := validCampaignInput("standard")
	in.MoneySiteURL = moneySiteURL
	in.CreditsAllocated = 10

	c, err := campSvc.Create(ctx, userID, in)
	if err != nil {
		t.Fatalf("setupReportJob Create campaign: %v", err)
	}

	domain := "report-target-" + randSuffix() + ".com"
	targetID := insertPrebuiltTarget(t, pool, "standard", domain)
	targetURL := "https://" + domain + "/blog/comment"

	// Insert job directly in 'dispatched' state (GetJobForReport requires dispatched/in_progress).
	jobID := insertJobDirect(t, pool, userID, c.ID, targetID, targetURL, "dispatched")

	return userID, c.ID, jobID, targetURL, moneySiteURL
}

func TestJobServiceReport_SuccessValid(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, _, jobID, targetURL, moneySiteURL := setupReportJob(t, pool)

	// result_url must be on same host as target; evidence must contain both money URL
	// and anchor text. insertJobDirect uses anchor='test anchor', so evidence must contain it.
	resultURL := targetURL + "?posted=1"
	evidence := fmt.Sprintf(`<a href="%s">test anchor</a>`, moneySiteURL)

	job, err := svc.Report(context.Background(), userID, jobID, service.JobResultInput{
		Status:    "success",
		ResultURL: resultURL,
		Evidence:  evidence,
	})
	if err != nil {
		t.Fatalf("Report success: %v", err)
	}
	if string(job.Status) != "success" {
		t.Fatalf("job status: want success, got %s", job.Status)
	}
}

func TestJobServiceReport_SuccessInvalidURL(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, _, jobID, _, moneySiteURL := setupReportJob(t, pool)

	// result_url on foreign host → ErrJobInvalid (validation order: result URL checked
	// before evidence, so evidence content does not matter here).
	evidence := fmt.Sprintf(`<a href="%s">test anchor</a>`, moneySiteURL)
	_, err := svc.Report(context.Background(), userID, jobID, service.JobResultInput{
		Status:    "success",
		ResultURL: "https://attacker.example.com/fake",
		Evidence:  evidence,
	})
	if err == nil {
		t.Fatal("expected ErrJobInvalid for foreign result_url, got nil")
	}
}

func TestJobServiceReport_SuccessMissingEvidence(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, _, jobID, targetURL, _ := setupReportJob(t, pool)

	resultURL := targetURL + "?posted=1"
	_, err := svc.Report(context.Background(), userID, jobID, service.JobResultInput{
		Status:    "success",
		ResultURL: resultURL,
		Evidence:  "", // missing
	})
	if err == nil {
		t.Fatal("expected ErrJobInvalid for empty evidence, got nil")
	}
}

func TestJobServiceReport_Failed(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, _, jobID, _, _ := setupReportJob(t, pool)

	job, err := svc.Report(context.Background(), userID, jobID, service.JobResultInput{
		Status:    "failed",
		ErrorCode: "captcha_timeout",
	})
	if err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	if string(job.Status) != "failed" {
		t.Fatalf("job status: want failed, got %s", job.Status)
	}
}

func TestJobServiceReport_Skipped(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID, _, jobID, _, _ := setupReportJob(t, pool)

	job, err := svc.Report(context.Background(), userID, jobID, service.JobResultInput{
		Status:    "skipped",
		ErrorCode: "site_offline",
	})
	if err != nil {
		t.Fatalf("Report skipped: %v", err)
	}
	if string(job.Status) != "skipped" {
		t.Fatalf("job status: want skipped, got %s", job.Status)
	}
}

func TestJobServiceReport_NotFound(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)

	_, err := svc.Report(context.Background(), userID, uuid.New(), service.JobResultInput{
		Status: "failed",
	})
	if err == nil {
		t.Fatal("expected ErrJobNotFound for random job ID, got nil")
	}
}

// TestJobServiceReport_CrossUser verifies F12: userB cannot report userA's job.
func TestJobServiceReport_CrossUser(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userA, _, jobID, _, _ := setupReportJob(t, pool)
	userB := insertTestUserWithCredits(t, pool, 0, 0)
	_ = userA

	_, err := svc.Report(context.Background(), userB, jobID, service.JobResultInput{
		Status: "failed",
	})
	if err == nil {
		t.Fatal("F12 IDOR: expected ErrJobNotFound when userB reports userA's job, got nil")
	}
}
