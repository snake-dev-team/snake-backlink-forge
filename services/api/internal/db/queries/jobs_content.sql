-- Queries for AI content generation on jobs.
-- These queries are separate from jobs.sql to keep the file size manageable.

-- name: UpdateJobContent :exec
-- Transitions a queued job to content_ready after AI generation.
-- Idempotent: only updates jobs still in queued status (skips already-processed).
UPDATE jobs
SET
    content_body  = sqlc.arg(content_body)::text,
    content_title = sqlc.arg(content_title)::text,
    content_meta  = sqlc.arg(content_meta)::text,
    status        = 'content_ready'
WHERE id      = sqlc.arg(id)::uuid
  AND user_id = sqlc.arg(user_id)::uuid
  AND status  = 'queued';

-- name: GetCampaignQueuedJobsForContent :many
-- Returns queued jobs for a campaign that have no content yet.
-- Used by ContentService to batch-generate articles.
SELECT
    j.id,
    j.anchor_text,
    j.anchor_type
FROM jobs j
JOIN campaigns c ON c.id = j.campaign_id
WHERE j.campaign_id = sqlc.arg(campaign_id)::uuid
  AND j.user_id     = sqlc.arg(user_id)::uuid
  AND j.status      = 'queued'
  AND j.content_body IS NULL
ORDER BY j.created_at ASC;
