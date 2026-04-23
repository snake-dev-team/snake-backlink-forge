-- Queries for the audit_log and safeguard_hits tables.
-- Phase 2+ will add real queries here (event insert, anomaly lookup).
-- ip_hash stores sha256(ip) — raw IP is never persisted (see docs/threat-model.md Phase 7).
-- Placeholder kept so sqlc can parse this file without errors.

-- name: PlaceholderAuditSelect :one
SELECT 1 AS dummy;
