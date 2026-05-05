-- Queries for support_tickets table. Phase 07: ticket insert + open-count cap.

-- name: InsertSupportTicket :one
INSERT INTO support_tickets (user_id, subject, body) VALUES ($1, $2, $3) RETURNING *;

-- name: CountOpenTicketsByUser :one
SELECT COUNT(*) FROM support_tickets WHERE user_id = $1 AND status IN ('open','in_progress');
