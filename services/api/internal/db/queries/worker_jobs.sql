-- Worker job queries for the embedded Go background worker (Phase 7.05).
-- These queries are separate from jobs.sql to keep file size manageable.
-- All claiming uses FOR UPDATE SKIP LOCKED for safe concurrent polling.

-- name: ClaimContentReadyJobs :many
-- Claims a batch of content_ready jobs atomically, setting in_progress + lease.
-- Uses FOR UPDATE SKIP LOCKED so concurrent worker goroutines never double-claim.
-- sqlc.arg() with explicit ::type cast — lesson from Phase 7.02/7.03/7.04.
WITH claimed AS (
  SELECT id FROM jobs
  WHERE status = 'content_ready'
    AND (lease_until IS NULL OR lease_until < NOW())
  ORDER BY created_at ASC
  LIMIT sqlc.arg(limit_count)::int
  FOR UPDATE SKIP LOCKED
)
UPDATE jobs
SET status      = 'in_progress',
    lease_until  = NOW() + (sqlc.arg(lease_seconds)::int * INTERVAL '1 second'),
    lease_holder = sqlc.arg(lease_holder)::text
FROM claimed
WHERE jobs.id = claimed.id
RETURNING jobs.*;

-- name: IncrementJobRetry :exec
-- Bumps retry_count and records last failure details without changing status.
-- Called before RescheduleJobRetry or MoveJobToDLQ to record the failure.
UPDATE jobs
SET retry_count   = retry_count + 1,
    last_error_at = NOW(),
    error_code    = sqlc.arg(error_code)::text,
    error_message = sqlc.arg(error_message)::text
WHERE id = sqlc.arg(job_id)::uuid;

-- name: MoveJobToDLQ :exec
-- Transitions a job to 'dlq' status after max retries exceeded.
-- Sets completed_at so it appears as a terminal state in duration metrics.
UPDATE jobs
SET status       = 'dlq',
    completed_at = NOW()
WHERE id = sqlc.arg(job_id)::uuid
  AND status = 'in_progress';

-- name: RescheduleJobRetry :exec
-- Re-queues a transiently-failed job as content_ready with a backoff lease_until.
-- The worker will not pick it up until lease_until has passed.
UPDATE jobs
SET status       = 'content_ready',
    lease_until  = NOW() + (sqlc.arg(backoff_seconds)::int * INTERVAL '1 second'),
    lease_holder = NULL
WHERE id = sqlc.arg(job_id)::uuid
  AND status = 'in_progress';

-- name: GetWorkerHealth :one
-- Returns queue depth, in-flight count, and last completed timestamp.
-- Used by GET /health/worker to expose worker state without exposing job data.
SELECT
    COUNT(*) FILTER (WHERE status = 'content_ready'
                       AND (lease_until IS NULL OR lease_until < NOW()))::int  AS queue_depth,
    COUNT(*) FILTER (WHERE status = 'in_progress')::int                        AS in_flight,
    COUNT(*) FILTER (WHERE status = 'success'
                       AND completed_at > NOW() - INTERVAL '5 minutes')::int   AS completed_last_5min,
    COUNT(*) FILTER (WHERE status IN ('failed','dlq')
                       AND completed_at > NOW() - INTERVAL '5 minutes')::int   AS failed_last_5min,
    MAX(completed_at) FILTER (WHERE status = 'success')                        AS last_completed_at
FROM jobs;
