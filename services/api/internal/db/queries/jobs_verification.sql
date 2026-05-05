-- Verification queries for post-publish checking.
-- These are separate from jobs.sql to keep file sizes manageable.
-- All queries target jobs with status='success' that need link verification.

-- name: MarkJobVerified :exec
-- Updates verification result columns after a verification attempt.
-- Increments verification_attempts and sets verified/anchor_verified/verified_at.
UPDATE jobs
SET verified              = sqlc.arg(verified)::boolean,
    anchor_verified       = sqlc.arg(anchor_verified)::boolean,
    verification_attempts = verification_attempts + 1,
    verified_at           = NOW(),
    last_verification_at  = NOW(),
    verification_error    = sqlc.arg(verification_error)::text
WHERE id = sqlc.arg(job_id)::uuid;

-- name: GetUnverifiedJobs :many
-- Returns success jobs that need verification or re-verification.
-- Conditions:
--   - status = 'success' (only completed jobs are verified)
--   - verified IS DISTINCT FROM TRUE (not yet successfully verified)
--   - verification_attempts < 3 (under retry limit)
--   - created_at < NOW() - INTERVAL '1 minute' (debounce: allow DNS propagation)
--   - last_verification_at IS NULL OR last_verification_at < NOW() - INTERVAL '15 minutes'
--     (cooldown: don't hammer the same URL repeatedly)
SELECT id, user_id, campaign_id, result_url, anchor_text, verification_attempts
FROM jobs
WHERE status = 'success'
  AND verified IS DISTINCT FROM TRUE
  AND verification_attempts < 3
  AND created_at < NOW() - INTERVAL '1 minute'
  AND (last_verification_at IS NULL OR last_verification_at < NOW() - INTERVAL '15 minutes')
ORDER BY created_at ASC
LIMIT sqlc.arg(limit_count)::int;

-- name: GetJobForVerification :one
-- Fetches a single job's data needed by the verifier (result_url, money_site_url, anchor_text).
SELECT j.id, j.result_url, j.anchor_text, j.verification_attempts, c.money_site_url
FROM jobs j
JOIN campaigns c ON c.id = j.campaign_id
WHERE j.id = sqlc.arg(job_id)::uuid
  AND j.status = 'success';
