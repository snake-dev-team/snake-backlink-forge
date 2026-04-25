// support_service_test.go — Phase 07: integration tests for SupportService.
// Requires DATABASE_URL env var (live Postgres with Phase 02 schema).
// Each test uses a fresh user ID for isolation; no table truncation.
package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────── harness ──────────────────────────────────────

func newTestSupportService(t *testing.T, pool *pgxpool.Pool) *service.SupportService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	q := sqlcdb.New(pool)
	return service.NewSupportService(pool, q, log.Named("support_test"))
}

// ─────────────────────────── tests ────────────────────────────────────────

// TestCreateTicket_Success inserts a ticket and verifies the returned row.
func TestCreateTicket_Success(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestSupportService(t, pool)
	userID := insertTestUser(t, pool)

	ticket, err := svc.CreateTicket(context.Background(), userID, "Test subject", "Test body")
	if err != nil {
		t.Fatalf("CreateTicket: unexpected error: %v", err)
	}
	if ticket.ID == uuid.Nil {
		t.Error("CreateTicket: returned ticket has nil ID")
	}
	if ticket.UserID != userID {
		t.Errorf("CreateTicket: user_id mismatch: got %v want %v", ticket.UserID, userID)
	}
	if ticket.Body != "Test body" {
		t.Errorf("CreateTicket: body mismatch: got %q want %q", ticket.Body, "Test body")
	}
}

// TestCreateTicket_DefaultSubject verifies empty subject becomes "Support Ticket".
func TestCreateTicket_DefaultSubject(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestSupportService(t, pool)
	userID := insertTestUser(t, pool)

	ticket, err := svc.CreateTicket(context.Background(), userID, "", "body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.Subject == nil || *ticket.Subject != "Support Ticket" {
		t.Errorf("expected subject 'Support Ticket', got %v", ticket.Subject)
	}
}

// TestCreateTicket_BodyTruncated verifies a 5000-byte body is stored as 4096 bytes.
func TestCreateTicket_BodyTruncated(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestSupportService(t, pool)
	userID := insertTestUser(t, pool)

	longBody := strings.Repeat("a", 5000)
	ticket, err := svc.CreateTicket(context.Background(), userID, "subj", longBody)
	if err != nil {
		t.Fatalf("CreateTicket: unexpected error: %v", err)
	}
	if len(ticket.Body) != service.SupportTicketBodyMaxBytes {
		t.Errorf("body not truncated: got len=%d, want %d", len(ticket.Body), service.SupportTicketBodyMaxBytes)
	}
}

// TestCreateTicket_CapReached verifies the 4th open ticket returns ErrSupportTicketCapReached.
func TestCreateTicket_CapReached(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestSupportService(t, pool)
	userID := insertTestUser(t, pool)

	// Insert 3 open tickets (cap = 3).
	for i := 0; i < service.SupportTicketCap; i++ {
		_, err := svc.CreateTicket(context.Background(), userID, "subj", "body")
		if err != nil {
			t.Fatalf("ticket %d: unexpected error: %v", i+1, err)
		}
	}

	// 4th ticket must be rejected.
	_, err := svc.CreateTicket(context.Background(), userID, "subj", "body")
	if !errors.Is(err, service.ErrSupportTicketCapReached) {
		t.Errorf("4th ticket: expected ErrSupportTicketCapReached, got %v", err)
	}
}

// TestCountOpenTickets_NewUser verifies a new user starts with 0 open tickets.
func TestCountOpenTickets_NewUser(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestSupportService(t, pool)
	userID := insertTestUser(t, pool)

	count, err := svc.CountOpenTickets(context.Background(), userID)
	if err != nil {
		t.Fatalf("CountOpenTickets: unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("new user should have 0 open tickets, got %d", count)
	}
}

// TestCountOpenTickets_AfterResolve verifies resolved tickets are not counted.
func TestCountOpenTickets_AfterResolve(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestSupportService(t, pool)
	userID := insertTestUser(t, pool)

	// Insert a ticket, then resolve it.
	ticket, err := svc.CreateTicket(context.Background(), userID, "subj", "body")
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	_, err = pool.Exec(context.Background(),
		`UPDATE support_tickets SET status = 'resolved' WHERE id = $1`, ticket.ID)
	if err != nil {
		t.Fatalf("resolve ticket: %v", err)
	}

	count, err := svc.CountOpenTickets(context.Background(), userID)
	if err != nil {
		t.Fatalf("CountOpenTickets: %v", err)
	}
	if count != 0 {
		t.Errorf("resolved ticket should not count as open, got %d", count)
	}
}
