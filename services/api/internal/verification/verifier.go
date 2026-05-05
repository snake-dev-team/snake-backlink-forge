package verification

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

const (
	// verifyChanBuffer is the number of job IDs that can be queued for verification
	// without blocking the caller. Sized to hold a full worker poll batch × 3 polls.
	verifyChanBuffer = 128

	// retryBatchSize is how many unverified jobs the ticker picks per tick.
	retryBatchSize = 50
)

// Verifier performs post-publish link verification asynchronously.
// Workers push job IDs into the channel; the Verifier processes them in the background.
// A ticker also re-checks previously unverified jobs (DNS propagation delay).
//
// Usage:
//
//	v := verification.New(q, log)
//	go v.Start(ctx)           // start background loop
//	v.Verify(jobID)           // called by worker after successful publish (non-blocking)
type Verifier struct {
	q          *sqlcdb.Queries
	log        *zap.Logger
	verifyChan chan uuid.UUID
}

// New constructs a Verifier. q must not be nil; log may be nil (falls back to nop).
func New(q *sqlcdb.Queries, log *zap.Logger) *Verifier {
	if log == nil {
		log = zap.NewNop()
	}
	return &Verifier{
		q:          q,
		log:        log.Named("verifier"),
		verifyChan: make(chan uuid.UUID, verifyChanBuffer),
	}
}

// Verify enqueues a job ID for async verification. Non-blocking: drops the request
// if the internal channel is full (a ticker will catch it in the next retry pass).
func (v *Verifier) Verify(jobID uuid.UUID) {
	select {
	case v.verifyChan <- jobID:
	default:
		// Channel full — ticker retry will pick it up.
		v.log.Warn("verifier channel full, job will be retried by ticker",
			zap.String("job_id", jobID.String()))
	}
}

// Start runs the verification loop until ctx is cancelled.
// Processes job IDs from the internal channel and runs a retry ticker every 5 minutes.
// Designed to be called as a goroutine: go v.Start(ctx).
func (v *Verifier) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	v.log.Info("verifier started")

	for {
		select {
		case <-ctx.Done():
			v.log.Info("verifier shutting down")
			return

		case jobID := <-v.verifyChan:
			// Process immediately after worker reports success.
			// Use a short background context so a slow site doesn't stall the loop.
			verCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			v.processOne(verCtx, jobID)
			cancel()

		case <-ticker.C:
			// Retry pass: pick jobs that need re-verification (DNS propagation lag).
			v.runRetryPass(ctx)
		}
	}
}

// processOne fetches the job's result_url and money_site_url, runs HTTPHead + VerifyAnchor,
// then writes the result back via MarkJobVerified.
func (v *Verifier) processOne(ctx context.Context, jobID uuid.UUID) {
	log := v.log.With(zap.String("job_id", jobID.String()))

	job, err := v.q.GetJobForVerification(ctx, jobID)
	if err != nil {
		log.Warn("GetJobForVerification failed — job may not be in success state", zap.Error(err))
		return
	}

	if job.ResultUrl == nil || *job.ResultUrl == "" {
		log.Warn("job has no result_url — skipping verification")
		v.markResult(ctx, jobID, boolPtr(false), boolPtr(false), "no result_url", log)
		return
	}

	resultURL := *job.ResultUrl

	// Step 1: HEAD check — is the URL reachable?
	statusCode, _, headErr := HTTPHead(ctx, resultURL)
	if headErr != nil || statusCode < 200 || statusCode >= 400 {
		errMsg := fmt.Sprintf("HEAD failed: status=%d", statusCode)
		if headErr != nil {
			errMsg = headErr.Error()
		}
		log.Info("verification: URL unreachable", zap.String("url", resultURL), zap.String("error", errMsg))
		v.markResult(ctx, jobID, boolPtr(false), boolPtr(false), errMsg, log)
		return
	}

	// Step 2: GET body + parse HTML for anchor match.
	anchorFound, anchorErr := VerifyAnchor(ctx, resultURL, job.MoneySiteUrl, job.AnchorText)
	if anchorErr != nil {
		// Page loaded but parsing failed — mark as live but anchor unknown.
		log.Warn("anchor check error", zap.String("url", resultURL), zap.Error(anchorErr))
		v.markResult(ctx, jobID, boolPtr(true), boolPtr(false), anchorErr.Error(), log)
		return
	}

	log.Info("verification complete",
		zap.String("url", resultURL),
		zap.Int("status", statusCode),
		zap.Bool("anchor_found", anchorFound),
	)
	v.markResult(ctx, jobID, boolPtr(true), boolPtr(anchorFound), "", log)
}

// runRetryPass queries for unverified success jobs and processes them in the background.
func (v *Verifier) runRetryPass(ctx context.Context) {
	jobs, err := v.q.GetUnverifiedJobs(ctx, retryBatchSize)
	if err != nil {
		v.log.Warn("GetUnverifiedJobs failed", zap.Error(err))
		return
	}
	if len(jobs) == 0 {
		return
	}

	v.log.Debug("retry pass: processing unverified jobs", zap.Int("count", len(jobs)))

	for _, job := range jobs {
		jobID := job.ID
		// Non-blocking enqueue — if channel is full, just skip (next tick will retry).
		select {
		case v.verifyChan <- jobID:
		default:
			v.log.Warn("retry pass: channel full, skipping job",
				zap.String("job_id", jobID.String()))
		}
	}
}

// markResult persists the verification outcome. errMsg="" means success; non-empty is stored as error.
// sqlc-generated MarkJobVerifiedParams uses non-nullable types: nil pointer → false / empty string.
func (v *Verifier) markResult(ctx context.Context, jobID uuid.UUID, verified, anchorVerified *bool, errMsg string, log *zap.Logger) {
	v1 := false
	if verified != nil {
		v1 = *verified
	}
	v2 := false
	if anchorVerified != nil {
		v2 = *anchorVerified
	}
	if err := v.q.MarkJobVerified(ctx, sqlcdb.MarkJobVerifiedParams{
		JobID:             jobID,
		Verified:          v1,
		AnchorVerified:    v2,
		VerificationError: errMsg,
	}); err != nil {
		log.Error("MarkJobVerified failed", zap.Error(err))
	}
}

// boolPtr converts a bool literal to a *bool for nullable DB fields.
func boolPtr(b bool) *bool { return &b }

// IsVerified returns true only when status 2xx is confirmed by HTTP HEAD.
// Exported so callers (service facade) can use the same definition.
func IsVerified(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
