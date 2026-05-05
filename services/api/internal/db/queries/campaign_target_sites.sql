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
