-- Queries for the transactions table. Phase 05: pending intent + cancel.
-- Phase 06 webhook will add MarkPaid / MarkRecovered queries.

-- name: InsertPendingTransaction :one
INSERT INTO transactions (user_id, provider, provider_ref, package_code, amount_vnd, premium_granted, standard_granted)
VALUES ($1, 'sepay', $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPendingTxByUserPackage :one
-- Returns the most-recent pending row for a user+package combination.
-- Used by CreateTopupIntent idempotent path when idx_tx_user_pkg_pending fires (23505).
SELECT * FROM transactions
WHERE user_id = $1 AND package_code = $2 AND status = 'pending'
ORDER BY created_at DESC LIMIT 1;

-- name: GetTxByProviderRef :one
-- Webhook lookup: find a transaction by its SePay order code (provider_ref).
SELECT * FROM transactions WHERE provider = 'sepay' AND provider_ref = $1 LIMIT 1;

-- name: CancelPendingTransaction :exec
-- [Q2] Sets status='cancelled' (NOT 'failed') so provider_ref stays in the
-- idx_tx_provider_ref_active partial index, enabling late-payment recovery in Phase 06.
-- Metadata is augmented (||) rather than replaced to preserve existing fields.
UPDATE transactions
SET status     = 'cancelled',
    updated_at = NOW(),
    metadata   = metadata || jsonb_build_object(
                     'cancel_reason', $2::text,
                     'cancelled_at',  to_char(NOW(), 'YYYY-MM-DD"T"HH24:MI:SSOF')
                 )
WHERE id = $1 AND status = 'pending';

-- name: GetTxByUserPage :many
SELECT * FROM transactions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountTxByUser :one
SELECT COUNT(*) FROM transactions WHERE user_id = $1;
