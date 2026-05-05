-- Queries for the targets table.
-- Note: {niche} in dork_patterns is a Go template placeholder, NOT SQL interpolation.

-- name: CreateCustomTarget :one
INSERT INTO targets (
    url, domain, tld, type, source, owner_user_id, pool,
    dr, da, traffic_est, language, niche_tags, platform,
    form_selectors, captcha_type, captcha_sitekey, is_active, is_blocklisted
) VALUES (
    $1, $2, $3, $4, 'custom', $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, TRUE, FALSE
)
RETURNING *;

-- name: ListTargetsForUser :many
SELECT * FROM targets
WHERE is_active
  AND NOT is_blocklisted
  AND (owner_user_id = $1 OR owner_user_id IS NULL)
  AND ($2::text = '' OR pool = $2)
  AND ($3::text = '' OR type = $3::target_type)
ORDER BY owner_user_id NULLS LAST, success_rate DESC, created_at DESC
LIMIT $4 OFFSET $5;

-- name: GetTargetForUser :one
SELECT * FROM targets
WHERE id = $2
  AND is_active
  AND NOT is_blocklisted
  AND (owner_user_id = $1 OR owner_user_id IS NULL);

-- name: PickTargetsForCampaign :many
SELECT t.* FROM targets t
WHERE t.is_active
  AND NOT t.is_blocklisted
  AND t.pool = $2
  AND (t.owner_user_id = $1 OR t.owner_user_id IS NULL)
  AND NOT EXISTS (
      SELECT 1 FROM jobs j
      WHERE j.campaign_id = $3 AND j.target_id = t.id
  )
  AND NOT EXISTS (
      SELECT 1 FROM domain_cooldown dc
      WHERE dc.user_id = $1
        AND dc.domain = t.domain
        AND dc.last_used > NOW() - ($4::int * INTERVAL '1 hour')
  )
ORDER BY t.owner_user_id NULLS LAST, t.success_rate DESC, t.created_at DESC
LIMIT $5
FOR UPDATE OF t SKIP LOCKED;

-- name: UpsertDomainCooldown :exec
INSERT INTO domain_cooldown (user_id, domain, last_used)
VALUES ($1, $2, NOW())
ON CONFLICT (user_id, domain) DO UPDATE SET last_used = EXCLUDED.last_used;
