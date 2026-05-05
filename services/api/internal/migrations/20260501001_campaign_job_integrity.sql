-- +goose Up

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_campaign_target_unique
    ON jobs(campaign_id, target_id);

-- +goose Down

DROP INDEX IF EXISTS idx_jobs_campaign_target_unique;
