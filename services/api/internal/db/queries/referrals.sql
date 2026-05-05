-- Queries for referrals table. Phase 07: code lookup + insert + increment.

-- name: GetReferralByUser :one
SELECT * FROM referrals WHERE user_id = $1;

-- name: GetReferralByCode :one
SELECT * FROM referrals WHERE code = $1;

-- name: InsertReferral :one
INSERT INTO referrals (user_id, code) VALUES ($1, $2) RETURNING *;

-- name: IncrementReferralCount :exec
UPDATE referrals SET total_referred = total_referred + 1 WHERE user_id = $1;
