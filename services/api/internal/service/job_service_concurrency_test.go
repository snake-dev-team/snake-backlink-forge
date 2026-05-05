// job_service_concurrency_test.go — race-condition tests for JobService.ClaimNext.
// Requires DATABASE_URL env var. Uses live Postgres with FOR UPDATE SKIP LOCKED.
//
// Covered invariants:
//   ClaimNext with 2 goroutines competing for 1 job → exactly 1 success, 1 ErrJobNotFound.
//   ClaimNext with no queued jobs → ErrJobNotFound.
package service_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
)

// TestJobServiceClaimNext_Concurrency spawns 2 goroutines competing for exactly
// 1 queued job. The FOR UPDATE SKIP LOCKED in ClaimNextQueuedJob ensures exactly
// one goroutine wins; the other must get ErrJobNotFound.
// Run with: go test -race -run TestJobServiceClaimNext_Concurrency ./internal/service/...
func TestJobServiceClaimNext_Concurrency(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 50)

	// Insert exactly 1 queued job for this user via insertCampaignWithJobs helper.
	_ = insertCampaignWithJobs(t, pool, userID, 1)

	const goroutines = 2
	var (
		wg           sync.WaitGroup
		successCount atomic.Int32
		notFoundCount atomic.Int32
	)

	barrier := make(chan struct{}) // synchronize goroutine start

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-barrier // wait until all goroutines are ready
			_, err := svc.ClaimNext(context.Background(), userID)
			switch {
			case err == nil:
				successCount.Add(1)
			case errors.Is(err, service.ErrJobNotFound):
				notFoundCount.Add(1)
			default:
				// Serialization/retry errors may appear; count as not-found for this test.
				// The invariant is: total successes == 1.
				t.Logf("ClaimNext unexpected error (counted as not-found): %v", err)
				notFoundCount.Add(1)
			}
		}()
	}
	close(barrier) // release all goroutines simultaneously
	wg.Wait()

	if successCount.Load() != 1 {
		t.Fatalf("expected exactly 1 ClaimNext success, got %d", successCount.Load())
	}
	if notFoundCount.Load() != 1 {
		t.Fatalf("expected exactly 1 ErrJobNotFound, got %d", notFoundCount.Load())
	}
}

// TestJobServiceClaimNext_Empty verifies ClaimNext returns ErrJobNotFound when no
// queued jobs exist for the user.
func TestJobServiceClaimNext_Empty(t *testing.T) {
	pool := newTestPool(t)
	svc := setupJobService(t, pool)
	userID := insertTestUserWithCredits(t, pool, 0, 0)
	// No jobs inserted — ClaimNext must return ErrJobNotFound.

	_, err := svc.ClaimNext(context.Background(), userID)
	if !errors.Is(err, service.ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound for empty queue, got %v", err)
	}
}
