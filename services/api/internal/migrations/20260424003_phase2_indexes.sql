-- +goose Up
-- +goose NO TRANSACTION

-- [F4] Partial unique index: one phone_e164 can have trial_used=TRUE at most once.
-- This IS the trial-phone-reuse gate — no COUNT pre-check needed.
-- SQLSTATE 23505 on this constraint → ErrTrialPhoneReused in user_service.go.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_trial
    ON users(phone_e164) WHERE trial_used = TRUE AND phone_e164 IS NOT NULL;

-- Prevent duplicate pending transactions for the same user+package (debounce double-tap).
CREATE UNIQUE INDEX IF NOT EXISTS idx_tx_user_pkg_pending
    ON transactions(user_id, package_code) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS support_tickets (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject     VARCHAR(128),
    body        TEXT NOT NULL,
    status      VARCHAR(16) NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'in_progress', 'resolved', 'closed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_support_open
    ON support_tickets(status, created_at DESC) WHERE status IN ('open', 'in_progress');

CREATE TABLE IF NOT EXISTS referrals (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    code           VARCHAR(16) NOT NULL UNIQUE,
    total_referred INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down

DROP TABLE IF EXISTS referrals;
DROP TABLE IF EXISTS support_tickets;
DROP INDEX IF EXISTS idx_tx_user_pkg_pending;
DROP INDEX IF EXISTS idx_users_phone_trial;
