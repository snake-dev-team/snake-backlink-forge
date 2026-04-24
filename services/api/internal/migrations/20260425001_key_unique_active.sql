-- +goose Up
-- +goose NO TRANSACTION

-- Business invariant: §1.3 — exactly 1 active key per user.
-- Enforced at DB level via partial UNIQUE index; KeyService.Issue catches
-- 23505 on concurrent race and recovers.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_keys_user_active_unique
  ON api_keys(user_id) WHERE is_active = TRUE;

-- Legacy non-unique index superseded for uniqueness purposes but kept for query performance.
-- idx_keys_user from 20260424001 supports WHERE user_id=$1 AND is_active=TRUE LIMIT 1 lookups.
-- Both indexes coexist; unique index enforces invariant, non-unique index aids fast lookup.
-- DO NOT drop idx_keys_user.

-- +goose Down
-- +goose NO TRANSACTION

DROP INDEX IF EXISTS idx_keys_user_active_unique;
