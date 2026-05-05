-- +goose Up

-- Add content fields for AI-generated article per job.
-- content_body  : full HTML body of the article
-- content_title : article title
-- content_meta  : meta description for SEO

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS content_title TEXT,
    ADD COLUMN IF NOT EXISTS content_meta  TEXT;

-- Add content_ready status to the job_status enum.
-- Must appear BEFORE 'dispatched' in the logical workflow:
--   queued → content_ready → dispatched → in_progress → success | failed | skipped
ALTER TYPE job_status ADD VALUE IF NOT EXISTS 'content_ready' BEFORE 'dispatched';

-- +goose Down

-- NOTE: Postgres does not support removing enum values.
-- The content_title / content_meta columns can be dropped safely.
ALTER TABLE jobs
    DROP COLUMN IF EXISTS content_title,
    DROP COLUMN IF EXISTS content_meta;
