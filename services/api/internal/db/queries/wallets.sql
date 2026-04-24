-- Queries for the wallets table. Phase 04: balance fetch + VND spend bump.

-- name: GetWalletByUser :one
SELECT * FROM wallets WHERE user_id = $1;

-- name: AddVNDSpent :exec
UPDATE wallets SET total_vnd_spent = total_vnd_spent + $2, updated_at = NOW()
WHERE user_id = $1;
