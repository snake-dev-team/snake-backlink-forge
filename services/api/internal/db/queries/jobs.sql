-- Queries for the jobs table.

-- name: CreateJob :one
INSERT INTO jobs (
    user_id, campaign_id, target_id, target_url_snapshot, anchor_text,
    anchor_type, content_body, status, pool, credits_cost
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, 'queued', $8, $9
)
ON CONFLICT (campaign_id, target_id) WHERE target_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: CreateJobForSite :one
-- Inserts a job sourced from a user's own wp_site (campaign_target_sites path).
-- target_id is NULL because wp_sites are not rows in the targets table.
-- Deduplication is enforced by the jobs_campaign_url_unique partial index
-- (campaign_id, target_url_snapshot) WHERE target_id IS NULL.
INSERT INTO jobs (
    user_id, campaign_id, target_id, target_url_snapshot, anchor_text,
    anchor_type, status, pool, credits_cost
) VALUES (
    sqlc.arg(user_id)::uuid,
    sqlc.arg(campaign_id)::uuid,
    NULL,
    sqlc.arg(target_url_snapshot)::text,
    sqlc.arg(anchor_text)::text,
    sqlc.arg(anchor_type)::text,
    'queued',
    sqlc.arg(pool)::text,
    sqlc.arg(credits_cost)::int
)
ON CONFLICT (campaign_id, target_url_snapshot) WHERE target_id IS NULL DO NOTHING
RETURNING *;

-- name: CountCampaignQueuedWork :one
SELECT COALESCE(SUM(credits_cost + captcha_cost), 0)::int
FROM jobs
WHERE user_id = $1
  AND campaign_id = $2
  AND status IN ('queued', 'dispatched', 'in_progress');

-- name: CountCampaignCompletedWork :one
SELECT COALESCE(SUM(credits_cost + captcha_cost), 0)::int
FROM jobs
WHERE user_id = $1
  AND campaign_id = $2
  AND status = 'success';

-- name: GetJobForReport :one
SELECT * FROM jobs
WHERE user_id = $1
  AND id = $2
  AND status IN ('dispatched', 'in_progress')
FOR UPDATE;

-- name: ListJobsByCampaign :many
SELECT * FROM jobs
WHERE user_id = $1 AND campaign_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetJobByUser :one
SELECT * FROM jobs
WHERE user_id = $1 AND id = $2;

-- name: ClaimNextQueuedJob :one
-- Phase 7.04: claims content_ready jobs first (AI content populated), falling
-- back to queued jobs (for backward compat when AI generation is disabled).
-- Extension receives content_title / content_meta alongside content_body.
WITH claimed AS (
    UPDATE jobs
    SET status = 'dispatched', dispatched_at = NOW()
    WHERE id = (
        SELECT j.id
        FROM jobs j
        JOIN campaigns c ON c.id = j.campaign_id
        WHERE j.user_id = $1
          AND j.status IN ('content_ready', 'queued')
          AND c.status = 'running'
        ORDER BY j.status DESC, j.created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    )
    RETURNING *
)
SELECT claimed.id, claimed.user_id, claimed.campaign_id, claimed.target_id,
       claimed.target_url_snapshot, claimed.anchor_text, claimed.anchor_type,
       claimed.content_body, claimed.content_title, claimed.content_meta,
       claimed.status, claimed.pool, claimed.credits_cost,
       claimed.captcha_cost, claimed.error_code, claimed.error_message,
       claimed.result_url, claimed.dispatched_at, claimed.completed_at,
       claimed.created_at, c.money_site_url
FROM claimed
JOIN campaigns c ON c.id = claimed.campaign_id;

-- name: MarkJobInProgress :one
UPDATE jobs
SET status = 'in_progress'
WHERE user_id = $1 AND id = $2 AND status IN ('queued', 'dispatched')
RETURNING *;

-- name: CompleteJob :one
UPDATE jobs
SET status = 'success', result_url = $3, completed_at = NOW(), error_code = NULL, error_message = NULL
WHERE user_id = $1 AND id = $2 AND status IN ('dispatched', 'in_progress')
RETURNING *;

-- name: FailJob :one
UPDATE jobs
SET status = 'failed', error_code = $3, error_message = $4, completed_at = NOW()
WHERE user_id = $1 AND id = $2 AND status IN ('queued', 'dispatched', 'in_progress')
RETURNING *;

-- name: SkipJob :one
UPDATE jobs
SET status = 'skipped', error_code = $3, error_message = $4, completed_at = NOW()
WHERE user_id = $1 AND id = $2 AND status IN ('queued', 'dispatched', 'in_progress')
RETURNING *;

-- name: CampaignJobStats :one
SELECT
    COUNT(*)::int AS total,
    COUNT(*) FILTER (WHERE status = 'queued')::int AS queued,
    COUNT(*) FILTER (WHERE status = 'dispatched')::int AS dispatched,
    COUNT(*) FILTER (WHERE status = 'in_progress')::int AS in_progress,
    COUNT(*) FILTER (WHERE status = 'success')::int AS success,
    COUNT(*) FILTER (WHERE status = 'failed')::int AS failed,
    COUNT(*) FILTER (WHERE status = 'skipped')::int AS skipped
FROM jobs
WHERE user_id = $1 AND campaign_id = $2;
