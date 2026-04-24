-- +goose Up
-- +goose NO TRANSACTION

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ─────────── USERS ────────────────────────────────────────────
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    telegram_id     BIGINT UNIQUE NOT NULL,
    telegram_username VARCHAR(64),
    phone_e164      VARCHAR(20),
    is_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    is_banned       BOOLEAN NOT NULL DEFAULT FALSE,
    trial_used      BOOLEAN NOT NULL DEFAULT FALSE,
    language        VARCHAR(8) NOT NULL DEFAULT 'vi',
    referred_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at  TIMESTAMPTZ
);
CREATE INDEX idx_users_telegram_id ON users(telegram_id);

-- ─────────── API KEYS ────────────────────────────────────────
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash        BYTEA NOT NULL UNIQUE,          -- SHA256(plaintext)
    key_prefix      VARCHAR(12) NOT NULL,           -- First 8 chars for display "sbf_live_Zk3p..."
    name            VARCHAR(64),                    -- User label
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ
);
CREATE INDEX idx_keys_user ON api_keys(user_id) WHERE is_active = TRUE;
CREATE INDEX idx_keys_hash ON api_keys(key_hash) WHERE is_active = TRUE;

-- ─────────── WALLETS ────────────────────────────────────────
CREATE TABLE wallets (
    user_id              UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    premium_credits      INT NOT NULL DEFAULT 0 CHECK (premium_credits >= 0),
    standard_credits     INT NOT NULL DEFAULT 0 CHECK (standard_credits >= 0),
    total_premium_spent  INT NOT NULL DEFAULT 0,
    total_standard_spent INT NOT NULL DEFAULT 0,
    total_vnd_spent      BIGINT NOT NULL DEFAULT 0,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────── LEDGER (immutable audit) ────────────────────────
CREATE TYPE ledger_event_type AS ENUM (
    'trial_grant',
    'topup',
    'consume_backlink',
    'consume_captcha',
    'consume_finder',
    'refund',
    'admin_adjust'
);

CREATE TABLE ledger (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id),
    event_type      ledger_event_type NOT NULL,
    pool            VARCHAR(16) NOT NULL CHECK (pool IN ('premium', 'standard')),
    delta_credits   INT NOT NULL,                   -- + for add, - for consume
    balance_after   INT NOT NULL,
    ref_entity_type VARCHAR(32),                    -- 'transaction', 'job', 'campaign'
    ref_entity_id   UUID,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ledger_user_time ON ledger(user_id, created_at DESC);
CREATE INDEX idx_ledger_ref ON ledger(ref_entity_type, ref_entity_id);

-- ─────────── TRANSACTIONS (top-ups) ──────────────────────────
CREATE TYPE transaction_status AS ENUM (
    'pending',
    'paid',
    'failed',
    'refunded',
    'manual_review'
);

CREATE TYPE transaction_provider AS ENUM (
    'sepay',
    'lemonsqueezy',
    'manual'
);

CREATE TABLE transactions (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id),
    provider          transaction_provider NOT NULL,
    provider_ref      VARCHAR(255) UNIQUE,          -- SePay transaction ID, LS order ID
    package_code      VARCHAR(64) NOT NULL,         -- 'standard_pro_200', 'premium_max_300', 'combo_p100_s50'
    amount_vnd        BIGINT NOT NULL CHECK (amount_vnd > 0),
    premium_granted   INT NOT NULL DEFAULT 0,
    standard_granted  INT NOT NULL DEFAULT 0,
    status            transaction_status NOT NULL DEFAULT 'pending',
    paid_at           TIMESTAMPTZ,
    metadata          JSONB NOT NULL DEFAULT '{}',  -- raw webhook payload
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tx_user ON transactions(user_id, created_at DESC);
CREATE INDEX idx_tx_provider_ref ON transactions(provider, provider_ref);
CREATE INDEX idx_tx_status ON transactions(status) WHERE status IN ('pending', 'manual_review');

-- ─────────── CAMPAIGNS ──────────────────────────────────────
CREATE TYPE campaign_status AS ENUM (
    'draft', 'running', 'paused', 'completed', 'archived'
);

CREATE TABLE campaigns (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(128) NOT NULL,
    money_site_url  TEXT NOT NULL,
    niche_keywords  TEXT[] NOT NULL DEFAULT '{}',   -- ['crypto wallet', 'airdrop']
    anchor_texts    JSONB NOT NULL DEFAULT '[]',    -- [{text, weight, type}]
    pool            VARCHAR(16) NOT NULL CHECK (pool IN ('premium', 'standard')),
    source_mode     VARCHAR(32) NOT NULL CHECK (source_mode IN ('prebuilt', 'autofind', 'custom', 'mixed')),
    daily_limit     INT NOT NULL DEFAULT 20 CHECK (daily_limit BETWEEN 5 AND 50),
    status          campaign_status NOT NULL DEFAULT 'draft',
    ethical_mode    BOOLEAN NOT NULL DEFAULT TRUE,
    niche_filter    BOOLEAN NOT NULL DEFAULT TRUE,
    credits_allocated INT NOT NULL DEFAULT 0,
    credits_consumed  INT NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_campaigns_user ON campaigns(user_id, status);

-- Max 3 active campaigns per user enforced at app layer + trigger below
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION check_active_campaign_limit()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'running' THEN
        IF (SELECT COUNT(*) FROM campaigns
            WHERE user_id = NEW.user_id
              AND status = 'running'
              AND id != NEW.id) >= 3 THEN
            RAISE EXCEPTION 'Max 3 active campaigns per user';
        END IF;
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_campaign_limit BEFORE INSERT OR UPDATE ON campaigns
    FOR EACH ROW EXECUTE FUNCTION check_active_campaign_limit();
-- +goose StatementEnd

-- ─────────── TARGETS (prebuilt pool + per-campaign custom) ─────────
CREATE TYPE target_type AS ENUM (
    'blog_comment', 'forum_profile', 'web2_post', 'directory_listing'
);

CREATE TYPE target_source AS ENUM ('prebuilt', 'autofind', 'custom');

CREATE TABLE targets (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url             TEXT NOT NULL,
    domain          TEXT NOT NULL,
    tld             VARCHAR(32),
    type            target_type NOT NULL,
    source          target_source NOT NULL,
    owner_user_id   UUID REFERENCES users(id) ON DELETE CASCADE, -- NULL if global prebuilt
    pool            VARCHAR(16) NOT NULL CHECK (pool IN ('premium', 'standard')),
    dr              INT,                           -- Domain Rating 0-100
    da              INT,                           -- Domain Authority 0-100
    traffic_est     INT,
    language        VARCHAR(8),
    niche_tags      TEXT[] NOT NULL DEFAULT '{}',
    platform        VARCHAR(32),                   -- 'wordpress', 'disqus', 'discourse', 'phpbb'
    form_selectors  JSONB,                         -- cached DOM selectors
    captcha_type    VARCHAR(32),                   -- 'none', 'recaptcha_v2', 'hcaptcha', 'cf_turnstile'
    captcha_sitekey TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    is_blocklisted  BOOLEAN NOT NULL DEFAULT FALSE,
    success_rate    NUMERIC(4,3) NOT NULL DEFAULT 0.500,
    last_verified_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(url, owner_user_id)
);
CREATE INDEX idx_targets_pool_type ON targets(pool, type) WHERE is_active AND NOT is_blocklisted;
CREATE INDEX idx_targets_owner ON targets(owner_user_id) WHERE owner_user_id IS NOT NULL;
CREATE INDEX idx_targets_domain ON targets(domain);
CREATE INDEX idx_targets_niche ON targets USING GIN(niche_tags);

-- ─────────── JOBS (each backlink attempt) ──────────────────────
CREATE TYPE job_status AS ENUM (
    'queued', 'dispatched', 'in_progress', 'success', 'failed', 'skipped'
);

CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    campaign_id     UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    target_id       UUID NOT NULL REFERENCES targets(id),
    target_url_snapshot TEXT NOT NULL,
    anchor_text     TEXT NOT NULL,
    anchor_type     VARCHAR(16) NOT NULL,          -- branded/naked/generic/exact
    content_body    TEXT,                          -- AI generated
    status          job_status NOT NULL DEFAULT 'queued',
    pool            VARCHAR(16) NOT NULL,
    credits_cost    INT NOT NULL DEFAULT 1,
    captcha_cost    INT NOT NULL DEFAULT 0,
    error_code      VARCHAR(64),
    error_message   TEXT,
    result_url      TEXT,                          -- final posted URL
    dispatched_at   TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_jobs_user_status ON jobs(user_id, status, created_at DESC);
CREATE INDEX idx_jobs_campaign ON jobs(campaign_id);
CREATE INDEX idx_jobs_target ON jobs(target_id);
CREATE INDEX idx_jobs_pending ON jobs(status) WHERE status IN ('queued', 'dispatched', 'in_progress');

-- ─────────── DOMAIN COOLDOWN (prevent spam same domain) ─────
CREATE TABLE domain_cooldown (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain      TEXT NOT NULL,
    last_used   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, domain)
);
CREATE INDEX idx_cooldown_time ON domain_cooldown(last_used);

-- ─────────── DORK PATTERNS (prebuilt seed) ──────────────────
CREATE TABLE dork_patterns (
    id           SERIAL PRIMARY KEY,
    pattern      TEXT NOT NULL,                    -- 'site:wordpress.com "{niche}" "leave a comment"'
    target_type  target_type NOT NULL,
    expected_platform VARCHAR(32),
    success_weight NUMERIC(4,3) DEFAULT 0.500,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────── AUDIT LOG (security events) ────────────────────
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID REFERENCES users(id),
    key_id      UUID REFERENCES api_keys(id),
    event       VARCHAR(64) NOT NULL,              -- 'key_generated', 'key_revoked', 'anomaly_detected'
    ip_hash     BYTEA,                             -- sha256(ip), never store raw IP
    country     VARCHAR(2),
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_user ON audit_log(user_id, created_at DESC);

-- ─────────── SAFEGUARD VIOLATIONS ──────────────────────────
CREATE TABLE safeguard_hits (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id),
    rule        VARCHAR(64) NOT NULL,              -- 'per_domain_limit', 'anchor_diversity'
    severity    VARCHAR(16) NOT NULL,              -- 'info', 'warn', 'block'
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────── FUNCTIONS: atomic credit consume ──────────────
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION consume_credits(
    p_user_id UUID,
    p_pool VARCHAR,
    p_amount INT,
    p_event_type ledger_event_type,
    p_ref_type VARCHAR,
    p_ref_id UUID
) RETURNS INT AS $$
DECLARE
    new_balance INT;
BEGIN
    IF p_pool = 'premium' THEN
        UPDATE wallets
        SET premium_credits = premium_credits - p_amount,
            total_premium_spent = total_premium_spent + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id AND premium_credits >= p_amount
        RETURNING premium_credits INTO new_balance;
    ELSE
        UPDATE wallets
        SET standard_credits = standard_credits - p_amount,
            total_standard_spent = total_standard_spent + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id AND standard_credits >= p_amount
        RETURNING standard_credits INTO new_balance;
    END IF;

    IF new_balance IS NULL THEN
        RAISE EXCEPTION 'INSUFFICIENT_CREDITS' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO ledger (user_id, event_type, pool, delta_credits, balance_after, ref_entity_type, ref_entity_id)
    VALUES (p_user_id, p_event_type, p_pool, -p_amount, new_balance, p_ref_type, p_ref_id);

    RETURN new_balance;
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION grant_credits(
    p_user_id UUID,
    p_pool VARCHAR,
    p_amount INT,
    p_event_type ledger_event_type,
    p_ref_type VARCHAR,
    p_ref_id UUID
) RETURNS INT AS $$
DECLARE
    new_balance INT;
BEGIN
    IF p_pool = 'premium' THEN
        UPDATE wallets
        SET premium_credits = premium_credits + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id
        RETURNING premium_credits INTO new_balance;
    ELSE
        UPDATE wallets
        SET standard_credits = standard_credits + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id
        RETURNING standard_credits INTO new_balance;
    END IF;

    INSERT INTO ledger (user_id, event_type, pool, delta_credits, balance_after, ref_entity_type, ref_entity_id)
    VALUES (p_user_id, p_event_type, p_pool, p_amount, new_balance, p_ref_type, p_ref_id);

    RETURN new_balance;
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- ─────────── VIEWS ──────────────────────────────────────────
CREATE VIEW v_user_stats AS
SELECT
    u.id,
    u.telegram_id,
    w.premium_credits,
    w.standard_credits,
    COUNT(DISTINCT c.id) FILTER (WHERE c.status = 'running') AS active_campaigns,
    COUNT(DISTINCT j.id) FILTER (WHERE j.status = 'success') AS total_backlinks,
    COUNT(DISTINCT j.id) FILTER (WHERE j.status = 'success' AND j.created_at > NOW() - INTERVAL '24h') AS backlinks_24h
FROM users u
LEFT JOIN wallets w ON w.user_id = u.id
LEFT JOIN campaigns c ON c.user_id = u.id
LEFT JOIN jobs j ON j.user_id = u.id
GROUP BY u.id, w.premium_credits, w.standard_credits;


-- +goose Down
-- +goose NO TRANSACTION

DROP VIEW IF EXISTS v_user_stats;

-- +goose StatementBegin
DROP FUNCTION IF EXISTS grant_credits(UUID, VARCHAR, INT, ledger_event_type, VARCHAR, UUID);
-- +goose StatementEnd

-- +goose StatementBegin
DROP FUNCTION IF EXISTS consume_credits(UUID, VARCHAR, INT, ledger_event_type, VARCHAR, UUID);
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_campaign_limit ON campaigns;

-- +goose StatementBegin
DROP FUNCTION IF EXISTS check_active_campaign_limit() CASCADE;
-- +goose StatementEnd

-- Drop tables in FK dependency reverse order
DROP TABLE IF EXISTS safeguard_hits;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS dork_patterns;
DROP TABLE IF EXISTS domain_cooldown;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS targets;
DROP TABLE IF EXISTS campaigns;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS ledger;
DROP TABLE IF EXISTS wallets;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;

-- Drop enums (after tables so FK deps are gone)
DROP TYPE IF EXISTS job_status;
DROP TYPE IF EXISTS target_source;
DROP TYPE IF EXISTS target_type;
DROP TYPE IF EXISTS campaign_status;
DROP TYPE IF EXISTS transaction_provider;
DROP TYPE IF EXISTS transaction_status;
DROP TYPE IF EXISTS ledger_event_type;

-- Extensions NOT dropped intentionally: uuid-ossp, pgcrypto, citext, pg_trgm
-- may be used by other apps sharing the same Postgres instance.
