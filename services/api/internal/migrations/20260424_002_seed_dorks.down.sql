-- +goose Down
-- Safe delete scoped to seed date; preserves any user-added patterns created after this migration.
DELETE FROM dork_patterns WHERE created_at >= '2026-04-24' AND created_at < '2026-04-25';
