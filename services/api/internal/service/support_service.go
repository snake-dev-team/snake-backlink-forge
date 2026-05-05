// support_service.go — Phase 07: support ticket creation with cap enforcement.
// Tickets are stored in support_tickets table (Phase 02 migration).
// Cap: max 3 open/in_progress tickets per user (anti-spam).
// Body truncation: 4096 bytes (matches Telegram per-message limit).
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// SupportTicketBodyMaxBytes is the maximum allowed byte length for ticket body.
// Matches Telegram's per-message character cap to prevent unbounded DB writes.
const SupportTicketBodyMaxBytes = 4096

// SupportTicketCap is the max number of open/in_progress tickets a user may have simultaneously.
const SupportTicketCap = 3

// ErrSupportTicketCapReached is returned when the user already has 3 open tickets.
var ErrSupportTicketCapReached = errors.New("ticket cap reached (3 open tickets per user)")

// SupportService handles support ticket lifecycle.
type SupportService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	log  *zap.Logger
}

// NewSupportService constructs a SupportService.
func NewSupportService(pool *pgxpool.Pool, q *sqlcdb.Queries, log *zap.Logger) *SupportService {
	return &SupportService{pool: pool, q: q, log: log}
}

// CountOpenTickets returns the number of open/in_progress tickets for a user.
func (s *SupportService) CountOpenTickets(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := s.q.CountOpenTicketsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// CreateTicket inserts a new support ticket.
//
// Rules enforced before insert:
//   - Body truncated to SupportTicketBodyMaxBytes (byte boundary — truncated UTF-8
//     sequences render as replacement chars in admin UI; acceptable per spec M5).
//   - Subject defaults to "Support Ticket" when empty.
//   - Returns ErrSupportTicketCapReached if user has >= SupportTicketCap open tickets.
func (s *SupportService) CreateTicket(ctx context.Context, userID uuid.UUID, subject, body string) (sqlcdb.SupportTicket, error) {
	// Enforce open-ticket cap before insert.
	count, err := s.CountOpenTickets(ctx, userID)
	if err != nil {
		s.log.Error("SupportService.CreateTicket: CountOpenTickets failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		return sqlcdb.SupportTicket{}, err
	}
	if count >= SupportTicketCap {
		return sqlcdb.SupportTicket{}, ErrSupportTicketCapReached
	}

	// Truncate body at byte boundary (M5: simple clamp, UTF-8 awareness YAGNI here).
	if len(body) > SupportTicketBodyMaxBytes {
		body = body[:SupportTicketBodyMaxBytes]
	}

	// Default subject.
	if subject == "" {
		subject = "Support Ticket"
	}
	subjectPtr := &subject

	ticket, err := s.q.InsertSupportTicket(ctx, sqlcdb.InsertSupportTicketParams{
		UserID:  userID,
		Subject: subjectPtr,
		Body:    body,
	})
	if err != nil {
		s.log.Error("SupportService.CreateTicket: InsertSupportTicket failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		return sqlcdb.SupportTicket{}, err
	}

	s.log.Info("SupportService.CreateTicket: created",
		zap.String("user_id", userID.String()),
		zap.String("ticket_id", ticket.ID.String()))
	return ticket, nil
}
