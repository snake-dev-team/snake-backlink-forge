-- Queries for the ledger table. Phase 04: paginated history + count.
-- grant_credits / consume_credits are stored procs called via tx.QueryRow, not sqlc.

-- name: GetLedgerPage :many
SELECT * FROM ledger WHERE user_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountLedgerByUser :one
SELECT COUNT(*) FROM ledger WHERE user_id = $1;

-- name: GetLedgerConsumesByUserPage :many
SELECT * FROM ledger
WHERE user_id = $1
  AND event_type IN ('consume_backlink', 'consume_captcha', 'consume_finder')
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountLedgerConsumesByUser :one
SELECT COUNT(*) FROM ledger
WHERE user_id = $1
  AND event_type IN ('consume_backlink', 'consume_captcha', 'consume_finder');

-- name: CountCreditsConsumedThisMonth :one
-- Uses Asia/Ho_Chi_Minh timezone for month boundary (VN-only user base).
-- Avoids UTC drift where VN users see counter reset 7 hours early at month end.
SELECT COALESCE(SUM(-delta_credits)::bigint, 0)::bigint AS consumed
FROM ledger
WHERE user_id = $1
  AND delta_credits < 0
  AND created_at >= (date_trunc('month', NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh') AT TIME ZONE 'Asia/Ho_Chi_Minh');
