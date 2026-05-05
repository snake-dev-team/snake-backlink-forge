-- +goose Up

-- Worker lease fields for server-side goroutine job claiming (Phase 7.05).
-- lease_until   : timestamp when the in-progress claim expires (worker heartbeat extends it)
-- lease_holder  : opaque worker identity string (hostname-pid) for observability
-- retry_count   : how many times this job has been retried after transient WP server errors
-- last_error_at : timestamp of the most recent retry failure (debugging aid)

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS lease_until   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS lease_holder  TEXT,
    ADD COLUMN IF NOT EXISTS retry_count   INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;

-- Add 'dlq' (dead-letter queue) to the job_status enum.
-- Jobs with retry_count >= 3 are moved here instead of 'failed'
-- so they can be distinguished from first-attempt failures.
ALTER TYPE job_status ADD VALUE IF NOT EXISTS 'dlq';

-- Index for efficient worker poll: only scans content_ready jobs with expired/null leases.
CREATE INDEX IF NOT EXISTS idx_jobs_worker_poll
    ON jobs (status, lease_until, created_at ASC)
    WHERE status = 'content_ready';

-- +goose Down

-- NOTE: Postgres does not support removing enum values once added.
-- Drop the index and worker columns only.
DROP INDEX IF EXISTS idx_jobs_worker_poll;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS lease_holder,
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS last_error_at;
