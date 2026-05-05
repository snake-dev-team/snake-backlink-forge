-- Queries for the campaigns table.
-- trg_campaign_limit trigger enforces max 3 running campaigns at DB layer.

-- name: CreateCampaign :one
INSERT INTO campaigns (
    user_id, name, money_site_url, niche_keywords, anchor_texts,
    pool, source_mode, daily_limit, ethical_mode, niche_filter,
    credits_allocated, status, started_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12,
    CASE WHEN $12 = 'running' THEN NOW() ELSE NULL END
)
RETURNING *;

-- name: ListCampaignsByUser :many
SELECT
    c.*,
    COUNT(j.id)::int AS job_count,
    COUNT(j.id) FILTER (WHERE j.status = 'queued')::int AS queued_count,
    COUNT(j.id) FILTER (WHERE j.status = 'dispatched')::int AS dispatched_count,
    COUNT(j.id) FILTER (WHERE j.status = 'in_progress')::int AS in_progress_count,
    COUNT(j.id) FILTER (WHERE j.status = 'success')::int AS success_count,
    COUNT(j.id) FILTER (WHERE j.status = 'failed')::int AS failed_count,
    COUNT(j.id) FILTER (WHERE j.status = 'skipped')::int AS skipped_count
FROM campaigns c
LEFT JOIN jobs j ON j.campaign_id = c.id
WHERE c.user_id = $1
GROUP BY c.id
ORDER BY c.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetCampaignByUser :one
SELECT * FROM campaigns
WHERE user_id = $1 AND id = $2;

-- name: UpdateCampaignStatus :one
UPDATE campaigns
SET status = $3,
    started_at = CASE WHEN $3 = 'running' AND started_at IS NULL THEN NOW() ELSE started_at END,
    completed_at = CASE WHEN $3 IN ('completed', 'archived') THEN NOW() ELSE completed_at END,
    updated_at = NOW()
WHERE user_id = $1 AND id = $2
RETURNING *;

-- name: BumpCampaignCreditsConsumed :exec
UPDATE campaigns
SET credits_consumed = credits_consumed + $3,
    updated_at = NOW()
WHERE user_id = $1 AND id = $2;

-- name: CountRunningCampaignsByUser :one
SELECT COUNT(*) FROM campaigns WHERE user_id = $1 AND status = 'running';

-- name: ResolveCampaignIDPrefix :many
-- Resolves a UUID prefix to up to 2 candidates for ambiguity check.
-- Bot uses 4-8 char prefixes; LIMIT 2 lets us detect collisions cheaply.
SELECT id, name FROM campaigns
WHERE user_id = $1 AND id::text LIKE sqlc.arg(prefix)::text || '%'
LIMIT 2;
