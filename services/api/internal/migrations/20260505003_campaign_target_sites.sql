-- +goose Up

CREATE TABLE campaign_target_sites (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    wp_site_id  UUID NOT NULL REFERENCES wp_sites(id) ON DELETE CASCADE,
    PRIMARY KEY (campaign_id, wp_site_id)
);

CREATE INDEX idx_campaign_target_sites_campaign ON campaign_target_sites(campaign_id);

-- +goose Down

DROP INDEX IF EXISTS idx_campaign_target_sites_campaign;
DROP TABLE IF EXISTS campaign_target_sites;
