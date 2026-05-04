-- Queries for the campaigns table.
-- Phase 2+ will add real queries here (create, list, pause, resume, archive).
-- trg_campaign_limit trigger enforces max 3 running campaigns at DB layer.
-- Placeholder kept so sqlc can parse this file without errors.

-- name: PlaceholderCampaignsSelect :one
SELECT 1 AS dummy;

-- name: CountRunningCampaignsByUser :one
SELECT COUNT(*) FROM campaigns WHERE user_id = $1 AND status = 'running';
