-- +goose Up
-- Adds post-publish verification columns to the jobs table.
-- verified: NULL = not yet attempted, TRUE = URL reachable, FALSE = unreachable.
-- anchor_verified: NULL = not attempted, TRUE = anchor link found, FALSE = missing.
-- verification_attempts: how many times verifier has tried (max 3).
-- verified_at: timestamp of last verification attempt.
-- verification_error: last error message (e.g. "timeout", "404 Not Found").
-- last_verification_at: timestamp of last attempt (for cooldown enforcement).
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS verified              BOOLEAN,
    ADD COLUMN IF NOT EXISTS anchor_verified       BOOLEAN,
    ADD COLUMN IF NOT EXISTS verification_attempts INT         NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS verified_at           TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS verification_error    TEXT,
    ADD COLUMN IF NOT EXISTS last_verification_at  TIMESTAMPTZ;

-- Index to support the ticker query: unverified success jobs older than 1 minute.
CREATE INDEX IF NOT EXISTS idx_jobs_verification_pending
    ON jobs (status, verification_attempts, created_at)
    WHERE status = 'success' AND verified IS DISTINCT FROM TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_verification_pending;
ALTER TABLE jobs
    DROP COLUMN IF EXISTS last_verification_at,
    DROP COLUMN IF EXISTS verification_error,
    DROP COLUMN IF EXISTS verified_at,
    DROP COLUMN IF EXISTS verification_attempts,
    DROP COLUMN IF EXISTS anchor_verified,
    DROP COLUMN IF EXISTS verified;
