-- Queries for admin dashboard stats. Phase 08: read-only aggregates used by AdminService.Stats.
-- All queries are point-in-time reads; no writes here.

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CountUsersBanned :one
SELECT COUNT(*) FROM users WHERE is_banned = TRUE;

-- name: CountUsersVerified :one
SELECT COUNT(*) FROM users WHERE is_verified = TRUE;

-- name: CountUsersTrialUsed :one
SELECT COUNT(*) FROM users WHERE trial_used = TRUE;

-- name: CountActiveKeys :one
SELECT COUNT(*) FROM api_keys WHERE is_active = TRUE;

-- name: TxStats24h :one
SELECT
  COALESCE(SUM(amount_vnd) FILTER (WHERE status='paid' AND paid_at > NOW() - INTERVAL '24 hours'), 0)::bigint AS revenue_24h,
  COUNT(*) FILTER (WHERE status='paid' AND paid_at > NOW() - INTERVAL '24 hours') AS paid_24h,
  COUNT(*) FILTER (WHERE status='pending') AS pending,
  COUNT(*) FILTER (WHERE status='manual_review') AS manual_review
FROM transactions;

-- name: CreditsOutstanding :one
SELECT COALESCE(SUM(premium_credits),0)::bigint AS premium, COALESCE(SUM(standard_credits),0)::bigint AS standard FROM wallets;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone_e164 = $1 LIMIT 1;

-- name: GetUserByKeyPrefix :one
SELECT u.* FROM users u JOIN api_keys k ON k.user_id = u.id
WHERE k.key_prefix = $1 AND k.is_active = TRUE LIMIT 1;

-- name: BanUser :exec
UPDATE users SET is_banned = TRUE, updated_at = NOW() WHERE telegram_id = $1;

-- name: UnbanUser :exec
UPDATE users SET is_banned = FALSE, updated_at = NOW() WHERE telegram_id = $1;
