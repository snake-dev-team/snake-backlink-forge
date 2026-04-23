-- Queries for the ledger table.
-- Phase 2+ will add real queries here (history pagination, balance reconciliation).
-- consume_credits / grant_credits are stored procs called via pool.Exec, not sqlc.
-- Placeholder kept so sqlc can parse this file without errors.

-- name: PlaceholderLedgerSelect :one
SELECT 1 AS dummy;
