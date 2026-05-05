// service_test_harness_test.go — shared test harness helpers for package service_test.
// Provides: randSuffix, insertTestUserWithCredits, insertPrebuiltTarget,
//           insertCampaignWithJobs, validCampaignInput, setupCampaignService.
// Used by campaign_service_test.go and job_service_test.go.
// No DB setup here — helpers are called per-test with an already-acquired pool.
package service_test

import (
	"context"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// randSuffix returns an 8-char random hex string for unique-index safety.
func randSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b) //nolint:gosec // test-only
	return hex.EncodeToString(b)
}

// insertTestUserWithCredits inserts a user + wallet with explicit credit amounts.
// Distinct name avoids collision with insertTestUser(t, pool) in key_service_test.go.
func insertTestUserWithCredits(t *testing.T, pool *pgxpool.Pool, premium, standard int32) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (telegram_id) VALUES ($1) RETURNING id`,
		uniqueTgID(),
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insertTestUserWithCredits: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO wallets (user_id, premium_credits, standard_credits)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		   SET premium_credits  = wallets.premium_credits  + EXCLUDED.premium_credits,
		       standard_credits = wallets.standard_credits + EXCLUDED.standard_credits`,
		userID, premium, standard,
	)
	if err != nil {
		t.Fatalf("insertTestUserWithCredits wallet: %v", err)
	}
	return userID
}

// insertPrebuiltTarget inserts an active, non-blocklisted target in the given pool.
// owner_user_id=NULL marks it as a global (prebuilt) target available to all campaigns.
func insertPrebuiltTarget(t *testing.T, pool *pgxpool.Pool, poolName, domain string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	targetURL := "https://" + domain + "/blog/comment"
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO targets
			(url, domain, tld, type, source, owner_user_id, pool,
			 form_selectors, is_active, is_blocklisted)
		VALUES
			($1, $2, 'com', 'blog_comment', 'prebuilt', NULL, $3,
			 '{}', TRUE, FALSE)
		RETURNING id`,
		targetURL, domain, poolName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertPrebuiltTarget(%s): %v", domain, err)
	}
	return id
}

// insertCampaignWithJobs creates a running campaign with N queued jobs for setup convenience.
// Jobs are inserted directly (bypassing Enqueue wallet debit) so wallet credits are not consumed.
// Returns campaign ID.
func insertCampaignWithJobs(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, jobCount int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	svc := service.NewCampaignService(pool, q, log.Named("setup"))

	c, err := svc.Create(ctx, userID, validCampaignInput("standard"))
	if err != nil {
		t.Fatalf("insertCampaignWithJobs Create: %v", err)
	}
	// Set to running so ClaimNext's JOIN (campaigns.status='running') matches.
	_, err = svc.SetStatus(ctx, userID, c.ID, sqlcdb.CampaignStatusRunning)
	if err != nil {
		t.Fatalf("insertCampaignWithJobs SetStatus running: %v", err)
	}

	// Insert N targets + jobs directly, bypassing Enqueue's wallet/credit logic.
	for i := 0; i < jobCount; i++ {
		domain := "setup-" + randSuffix() + ".com"
		targetID := insertPrebuiltTarget(t, pool, "standard", domain)
		_, err = pool.Exec(ctx, `
			INSERT INTO jobs
				(user_id, campaign_id, target_id, target_url_snapshot, anchor_text,
				 anchor_type, status, pool, credits_cost)
			VALUES ($1, $2, $3, $4, 'test anchor', 'branded', 'queued', 'standard', 1)`,
			userID, c.ID, targetID, "https://"+domain+"/blog/comment",
		)
		if err != nil {
			t.Fatalf("insertCampaignWithJobs insert job %d: %v", i, err)
		}
	}
	return c.ID
}

// validCampaignInput returns a minimal valid CampaignInput for the given pool.
// Uses random name/URL to avoid unique-index collisions across tests.
func validCampaignInput(pool string) service.CampaignInput {
	return service.CampaignInput{
		Name:             "test-camp-" + randSuffix(),
		MoneySiteURL:     "https://test-" + randSuffix() + ".example.com",
		AnchorTexts:      []service.AnchorTextInput{{Text: "my anchor", Type: "branded", Weight: 1}},
		Pool:             pool,
		SourceMode:       "prebuilt",
		DailyLimit:       10,
		CreditsAllocated: 50,
	}
}

// setupCampaignService constructs a CampaignService backed by the given pool.
func setupCampaignService(t *testing.T, pool *pgxpool.Pool) *service.CampaignService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	return service.NewCampaignService(pool, q, log.Named("test-campaign"))
}
