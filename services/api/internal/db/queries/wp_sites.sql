-- Queries for connected WordPress sites.

-- name: InsertWpSite :one
INSERT INTO wp_sites (user_id, base_url, app_username, app_password_enc, label, status, last_validated_at, last_error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListWpSitesByUser :many
SELECT id, user_id, base_url, app_username, label, status, last_validated_at, last_error, created_at, updated_at
FROM wp_sites
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetWpSiteByID :one
SELECT *
FROM wp_sites
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: SoftDeleteWpSite :exec
UPDATE wp_sites
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: UpdateWpSiteStatus :exec
UPDATE wp_sites
SET status = $1, last_validated_at = NOW(), last_error = $2, updated_at = NOW()
WHERE id = $3 AND user_id = $4 AND deleted_at IS NULL;

-- name: CountWpSitesByUser :one
SELECT COUNT(*) FROM wp_sites WHERE user_id = $1 AND deleted_at IS NULL;
