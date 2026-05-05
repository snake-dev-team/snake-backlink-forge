// job_runner.go: Executes a single claimed job — fetches WP creds, publishes article,
// reports result back via JobService. Called concurrently from the worker pool.
package worker

import (
	"context"
	"errors"
	"net/url"
	"strings"

	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// runJob executes one job end-to-end:
//  1. Rate-limit per site domain (WaitForSlot).
//  2. Fetch decrypted WP credentials via WpSiteService.GetByDomainPlain.
//  3. Post article via WP REST API (PostArticle).
//  4. On success: call JobService.Report(success).
//  5. On transient failure (5xx / network): IncrementJobRetry → if <limit, RescheduleJobRetry; else MoveJobToDLQ.
//  6. On permanent failure (4xx auth/forbidden): IncrementJobRetry → MoveJobToDLQ immediately.
//
// Panics are caught by the caller's recover() in worker.go.
func (w *Worker) runJob(ctx context.Context, job sqlcdb.Job) {
	log := w.log.With(zap.String("job_id", job.ID.String()), zap.String("target", job.TargetUrlSnapshot))

	// --- Guard: content must be present for WP publish ---
	if job.ContentBody == nil || strings.TrimSpace(*job.ContentBody) == "" {
		log.Warn("job has no content_body — skipping")
		w.failJobPermanent(ctx, job, "wp_no_content", "content_body is empty", log)
		return
	}

	// --- Extract domain for rate limiting and creds lookup ---
	domain, err := extractDomain(job.TargetUrlSnapshot)
	if err != nil {
		log.Warn("cannot extract domain from target URL", zap.Error(err))
		w.failJobPermanent(ctx, job, "wp_invalid_url", err.Error(), log)
		return
	}

	// --- Per-site rate limit: min 30s gap ---
	if err := w.rateLimiter.WaitForSlot(ctx, domain); err != nil {
		// Context cancelled during wait — don't mark job failed; lease will expire naturally.
		log.Info("rate limiter wait cancelled", zap.Error(err))
		return
	}

	// --- Fetch decrypted WP credentials ---
	if w.deps.WpSiteSvc == nil {
		log.Error("WpSiteService not configured — worker cannot publish")
		w.failJobPermanent(ctx, job, "wp_svc_unavailable", "WpSiteService is nil", log)
		return
	}

	creds, err := w.deps.WpSiteSvc.GetByDomainPlain(ctx, job.UserID, domain)
	if errors.Is(err, service.ErrWPSiteNotFound) {
		log.Warn("no WP site credentials found for domain", zap.String("domain", domain))
		w.failJobPermanent(ctx, job, "wp_no_credentials", "no connected WP site for domain "+domain, log)
		return
	}
	if err != nil {
		log.Error("failed to fetch WP credentials", zap.Error(err))
		w.handleTransientError(ctx, job, "wp_creds_error", err.Error(), log)
		return
	}

	// --- Build article request ---
	title := ""
	if job.ContentTitle != nil {
		title = *job.ContentTitle
	}
	meta := ""
	if job.ContentMeta != nil {
		meta = *job.ContentMeta
	}
	postReq := WPPostRequest{
		Title:           title,
		ContentHTML:     *job.ContentBody,
		MetaDescription: meta,
	}

	// --- Publish to WordPress ---
	result, err := PostArticle(ctx, creds.BaseURL, creds.AppUsername, creds.AppPasswordPlain, postReq)
	if err != nil {
		var wpErr *WPClientError
		if errors.As(err, &wpErr) {
			switch wpErr.Code {
			case "wp_auth_failed", "wp_forbidden", "wp_rest_disabled", "wp_blocked", "wp_client_error":
				// Permanent failure — no retry.
				log.Warn("WP permanent error", zap.String("code", wpErr.Code), zap.String("msg", wpErr.Message))
				w.failJobPermanent(ctx, job, wpErr.Code, wpErr.Message, log)
				return
			default:
				// Transient: wp_server_error, wp_network_error — retry with backoff.
				log.Warn("WP transient error", zap.String("code", wpErr.Code), zap.String("msg", wpErr.Message))
				w.handleTransientError(ctx, job, wpErr.Code, wpErr.Message, log)
				return
			}
		}
		// Unknown error type — treat as transient.
		log.Error("unexpected WP error", zap.Error(err))
		w.handleTransientError(ctx, job, "wp_unknown_error", err.Error(), log)
		return
	}

	// --- Report success ---
	in := service.JobResultInput{
		Status:    "success",
		ResultURL: result.PostURL,
		Evidence:  result.Evidence,
	}
	if _, reportErr := w.deps.JobSvc.Report(ctx, job.UserID, job.ID, in); reportErr != nil {
		log.Error("failed to report job success", zap.Error(reportErr))
		// Don't retry the WP post — article is live. Log and move on.
		// The lease will expire and the job remains in_progress until manual review.
		return
	}

	log.Info("job completed successfully", zap.String("post_url", result.PostURL))
}

// failJobPermanent marks a job as DLQ immediately without retry (auth failures, missing creds).
// Uses IncrementJobRetry first so error_code/error_message are persisted, then MoveJobToDLQ.
func (w *Worker) failJobPermanent(ctx context.Context, job sqlcdb.Job, code, msg string, log *zap.Logger) {
	if err := w.deps.Q.IncrementJobRetry(ctx, sqlcdb.IncrementJobRetryParams{
		JobID:        job.ID,
		ErrorCode:    code,
		ErrorMessage: msg,
	}); err != nil {
		log.Error("IncrementJobRetry failed", zap.Error(err))
	}
	if err := w.deps.Q.MoveJobToDLQ(ctx, job.ID); err != nil {
		log.Error("MoveJobToDLQ failed", zap.Error(err))
	}
	log.Warn("job moved to DLQ (permanent failure)", zap.String("code", code))
}

// handleTransientError increments retry count. If retry_count < RetryLimit, re-queues with
// exponential backoff. If retry_count >= RetryLimit, moves to DLQ.
// retry_count in the Job struct reflects the count BEFORE this failure.
func (w *Worker) handleTransientError(ctx context.Context, job sqlcdb.Job, code, msg string, log *zap.Logger) {
	if err := w.deps.Q.IncrementJobRetry(ctx, sqlcdb.IncrementJobRetryParams{
		JobID:        job.ID,
		ErrorCode:    code,
		ErrorMessage: msg,
	}); err != nil {
		log.Error("IncrementJobRetry failed", zap.Error(err))
		return
	}

	// newRetryCount = current (pre-increment) + 1
	newRetryCount := int(job.RetryCount) + 1

	if newRetryCount >= w.cfg.RetryLimit {
		if err := w.deps.Q.MoveJobToDLQ(ctx, job.ID); err != nil {
			log.Error("MoveJobToDLQ failed", zap.Error(err))
		}
		log.Warn("job moved to DLQ after max retries",
			zap.String("code", code),
			zap.Int("retry_count", newRetryCount),
			zap.Int("limit", w.cfg.RetryLimit))
		return
	}

	backoffSec := BackoffSeconds(newRetryCount)
	if err := w.deps.Q.RescheduleJobRetry(ctx, sqlcdb.RescheduleJobRetryParams{
		JobID:         job.ID,
		BackoffSeconds: int32(backoffSec),
	}); err != nil {
		log.Error("RescheduleJobRetry failed", zap.Error(err))
		return
	}

	log.Info("job rescheduled for retry",
		zap.String("code", code),
		zap.Int("retry_count", newRetryCount),
		zap.Int("backoff_sec", backoffSec))
}

// extractDomain parses a target URL and returns the lowercase hostname.
func extractDomain(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return "", errors.New("invalid target URL: " + rawURL)
	}
	return strings.ToLower(u.Hostname()), nil
}
