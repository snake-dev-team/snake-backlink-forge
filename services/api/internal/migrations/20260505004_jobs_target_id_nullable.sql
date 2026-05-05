-- +goose Up
-- Makes jobs.target_id nullable so campaign_target_sites path can insert jobs
-- without a target row (user's own WP site replaces third-party targets).
-- Also replaces the (campaign_id, target_id) UNIQUE constraint with a partial
-- index that only enforces uniqueness when target_id IS NOT NULL (legacy path).

-- Drop the existing unique constraint name (created in init migration).
-- The constraint may be a named UNIQUE or an unnamed one backing an index.
-- We use a two-step approach: alter the column, then recreate the index.
ALTER TABLE jobs ALTER COLUMN target_id DROP NOT NULL;

-- Drop the unique constraint on (campaign_id, target_id) — it cannot handle NULLs
-- correctly for the new path (NULLs are never equal in UNIQUE indexes on most DBs).
-- Re-create as a partial unique index covering only rows where target_id IS NOT NULL.
-- This preserves the duplicate-prevention semantics of the legacy path.
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_campaign_id_target_id_key;
DROP INDEX IF EXISTS jobs_campaign_id_target_id_idx;

CREATE UNIQUE INDEX IF NOT EXISTS jobs_campaign_target_unique
    ON jobs (campaign_id, target_id)
    WHERE target_id IS NOT NULL;

-- For the wp_sites path, uniqueness is enforced by target_url_snapshot per campaign:
-- one job per (campaign_id, target_url_snapshot) combination.
CREATE UNIQUE INDEX IF NOT EXISTS jobs_campaign_url_unique
    ON jobs (campaign_id, target_url_snapshot)
    WHERE target_id IS NULL;

-- +goose Down
-- Revert: drop partial indexes, restore NOT NULL and original unique constraint.
DROP INDEX IF EXISTS jobs_campaign_url_unique;
DROP INDEX IF EXISTS jobs_campaign_target_unique;

-- Restore the original constraint (fails if any NULL target_id rows exist — clean DB assumed).
ALTER TABLE jobs ALTER COLUMN target_id SET NOT NULL;
ALTER TABLE jobs ADD CONSTRAINT jobs_campaign_id_target_id_key UNIQUE (campaign_id, target_id);
