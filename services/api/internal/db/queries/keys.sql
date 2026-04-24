-- Queries for the api_keys table.
-- Phase 03: real queries replacing placeholder stub.
-- key_hash is BYTEA (SHA-256 of plaintext). Plaintext is NEVER stored.

-- name: GetActiveKeyByUser :one
SELECT * FROM api_keys WHERE user_id = $1 AND is_active = TRUE LIMIT 1;

-- name: RevokeActiveKeysForUser :exec
UPDATE api_keys SET is_active = FALSE, revoked_at = NOW()
WHERE user_id = $1 AND is_active = TRUE;

-- name: InsertKey :one
INSERT INTO api_keys (user_id, key_hash, key_prefix, name, is_active)
VALUES ($1, $2, $3, $4, TRUE)
RETURNING *;

-- name: GetKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1 AND is_active = TRUE LIMIT 1;
