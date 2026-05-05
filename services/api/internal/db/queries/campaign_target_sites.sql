-- Queries for the campaign_target_sites join table.
-- Maps which WP sites a campaign should post backlinks to.
-- Normalized join table: campaign_id × wp_site_id (PK on both).

-- name: InsertCampaignTargetSite :exec
INSERT INTO campaign_target_sites (campaign_id, wp_site_id)
VALUES ($1, $2)
ON CONFLICT (campaign_id, wp_site_id) DO NOTHING;

-- name: ListCampaignTargetSites :many
SELECT campaign_id, wp_site_id
FROM campaign_target_sites
WHERE campaign_id = $1
ORDER BY wp_site_id;

-- name: DeleteCampaignTargetSites :exec
DELETE FROM campaign_target_sites
WHERE campaign_id = $1;

-- name: ListWpSitesForCampaign :many
-- Returns full WP site rows for a campaign's selected target sites.
-- Used by JobService.Enqueue to resolve site URLs when building jobs.
SELECT w.id, w.base_url, w.app_username, w.label, w.status
FROM wp_sites w
JOIN campaign_target_sites cts ON cts.wp_site_id = w.id
WHERE cts.campaign_id = $1
  AND w.deleted_at IS NULL
  AND w.status = 'connected'
ORDER BY w.created_at;

-- name: CountCampaignTargetSites :one
-- Returns the number of wp_sites linked to a campaign.
-- Used by JobService.Enqueue to decide between the new wp_sites path and legacy targets path.
SELECT COUNT(*)::int FROM campaign_target_sites
WHERE campaign_id = sqlc.arg(campaign_id)::uuid;

-- name: PickWpSitesForCampaign :many
-- Returns connected wp_sites linked to a campaign that do NOT already have a job
-- for this campaign (deduplication: one job per campaign×site URL pair).
-- Uses FOR UPDATE OF w SKIP LOCKED so concurrent Enqueue calls don't double-pick.
SELECT w.id, w.base_url, w.app_username
FROM campaign_target_sites cts
JOIN wp_sites w ON w.id = cts.wp_site_id
WHERE cts.campaign_id = sqlc.arg(campaign_id)::uuid
  AND w.user_id = sqlc.arg(user_id)::uuid
  AND w.deleted_at IS NULL
  AND w.status = 'connected'
  AND NOT EXISTS (
      SELECT 1 FROM jobs j
      WHERE j.campaign_id = sqlc.arg(campaign_id)::uuid
        AND j.target_url_snapshot = w.base_url
  )
ORDER BY w.created_at ASC
LIMIT sqlc.arg(limit_count)::int
FOR UPDATE OF w SKIP LOCKED;
