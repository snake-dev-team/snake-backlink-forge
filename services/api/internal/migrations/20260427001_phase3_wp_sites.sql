-- +goose Up
-- +goose StatementBegin
CREATE TYPE wp_site_status AS ENUM ('pending', 'connected', 'error');

CREATE TABLE wp_sites (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    base_url          VARCHAR(512) NOT NULL,
    app_username      VARCHAR(120) NOT NULL,
    app_password_enc  BYTEA NOT NULL,
    label             VARCHAR(120) NOT NULL DEFAULT '',
    status            wp_site_status NOT NULL DEFAULT 'pending',
    last_validated_at TIMESTAMPTZ,
    last_error        TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_wp_sites_user_url_active
    ON wp_sites(user_id, base_url) WHERE deleted_at IS NULL;

CREATE INDEX idx_wp_sites_user
    ON wp_sites(user_id) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_wp_sites_user;
DROP INDEX IF EXISTS idx_wp_sites_user_url_active;
DROP TABLE IF EXISTS wp_sites;
DROP TYPE IF EXISTS wp_site_status;
-- +goose StatementEnd
