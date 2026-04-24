-- Queries for the ledger table. Phase 04: paginated history + count.
-- grant_credits / consume_credits are stored procs called via tx.QueryRow, not sqlc.

-- name: GetLedgerPage :many
SELECT * FROM ledger WHERE user_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountLedgerByUser :one
SELECT COUNT(*) FROM ledger WHERE user_id = $1;
