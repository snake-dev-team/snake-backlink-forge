-- Queries for the users table.
-- Phase 02: real queries replacing placeholder stub.
-- NOTE: F4 trial-gate flow (SELECT FOR UPDATE + conditional UPDATE + grant_credits)
--       is inlined as raw pgx.Tx in user_service.go — sqlc cannot model that compound
--       transaction cleanly.

-- name: GetUserByTelegramID :one
SELECT * FROM users WHERE telegram_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpsertUserStub :one
INSERT INTO users (telegram_id, telegram_username, language)
VALUES ($1, $2, 'vi')
ON CONFLICT (telegram_id) DO UPDATE SET
    telegram_username = EXCLUDED.telegram_username,
    last_active_at    = NOW(),
    updated_at        = NOW()
RETURNING *;

-- name: SetPhoneAndVerify :one
UPDATE users
SET phone_e164  = $2,
    is_verified = TRUE,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: EnsureWallet :exec
INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING;

-- name: SetLanguage :exec
UPDATE users SET language = $2, updated_at = NOW() WHERE id = $1;
