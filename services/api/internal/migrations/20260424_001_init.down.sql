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
