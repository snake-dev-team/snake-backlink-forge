-- Queries for the audit_log table. Phase 08: real event insert + lookup queries.
-- ip_hash stores sha256(ip) — raw IP is never persisted.
-- Phase 2 placeholder removed and replaced with real queries below.

-- name: InsertAuditLog :one
INSERT INTO audit_log (user_id, key_id, event, ip_hash, country, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAuditLogByEventSince :many
SELECT * FROM audit_log WHERE event = $1 AND created_at > $2
ORDER BY created_at DESC LIMIT $3;

-- name: CountAuditLogByEventSince :one
SELECT COUNT(*) FROM audit_log WHERE event = $1 AND created_at > $2;
