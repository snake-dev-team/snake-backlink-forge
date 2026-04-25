// webhook_service.go — Phase 06 [F1][Q2]: atomic CAS-gated credit grant on SePay payment.
//
// ProcessPaidTransaction is the single money-flow function in Phase 2.
// Correctness invariants:
//   [F1] 3-way branch on transferAmount vs amount_vnd (underpaid / exact / overpaid).
//   [Q2] CAS widens to status IN ('pending','cancelled') — late payment recovery.
//   Idempotency: second call with same orderCode sees RowsAffected=0 → AlreadyProcessed.
//   Race-safety: FOR UPDATE inside CTE serialises concurrent webhooks for same provider_ref.
package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"go.uber.org/zap"
)

// ErrDeadLetterSentinel is returned by ProcessPaidTransaction when orderCode equals
// DeadLetterSentinel ("DEADLETTER_TRIGGER"). Used by integration tests to exercise the
// retry consumer dead-letter path without DB fault injection.
var ErrDeadLetterSentinel = errors.New("dead_letter_sentinel — test trigger")

// ProcessResult carries the outcome of ProcessPaidTransaction to the handler.
type ProcessResult struct {
	AlreadyProcessed bool
	Underpaid        bool
	UnknownOrder     bool
	Overpaid         bool
	BonusCredits     int
	WasCancelled     bool // true when status transitions to 'recovered_by_late_payment'
	UserID           uuid.UUID
	PackageCode      string
	Premium          int // base premium credits granted (from tx row)
	Standard         int // base standard credits granted (from tx row)
}

// WebhookService handles the SePay payment webhook business logic.
type WebhookService struct {
	pool         *pgxpool.Pool
	wallet       *WalletService
	cfg          *config.Config
	log          *zap.Logger
	adminAlertCh chan<- notify.AdminAlert
}

// NewWebhookService constructs a WebhookService. All fields required except adminAlertCh
// which may be nil in tests that don't exercise the alert path.
func NewWebhookService(
	pool *pgxpool.Pool,
	wallet *WalletService,
	cfg *config.Config,
	log *zap.Logger,
	adminAlertCh chan<- notify.AdminAlert,
) *WebhookService {
	return &WebhookService{
		pool:         pool,
		wallet:       wallet,
		cfg:          cfg,
		log:          log,
		adminAlertCh: adminAlertCh,
	}
}

// perCreditRate derives the per-credit VND rate and target pool for bonus calculation.
// For combo packages (premium+standard), excess credits go to the premium pool (always present).
// For single-pool packages, excess credits go to that pool.
// Returns ("standard", 0) when total credits is zero (degenerate package — no bonus possible).
func perCreditRate(premiumCr, standardCr int, amount int64) (pool string, rate int64) {
	total := int64(premiumCr + standardCr)
	if total == 0 {
		return "standard", 0
	}
	if premiumCr > 0 {
		// Combo or premium-only: route excess to premium pool.
		// Rate = amount / total_credits (weighted average, floor).
		return "premium", amount / total
	}
	return "standard", amount / total
}

// ProcessPaidTransaction is the atomic CAS-gated payment handler.
// See webhook_service_process.go for the full implementation split across two files.
func (s *WebhookService) ProcessPaidTransaction(
	ctx context.Context,
	orderCode string,
	p sepay.Payload,
) (ProcessResult, error) {
	// Test sentinel — allows integration tests to force dead-letter path without DB fault injection.
	if orderCode == DeadLetterSentinel {
		return ProcessResult{}, ErrDeadLetterSentinel
	}
	return s.processTransaction(ctx, orderCode, p)
}

// auditEvent is the internal shape passed to auditLog.
type auditEvent struct {
	Event    string
	UserID   *uuid.UUID
	Metadata map[string]any
}

// auditLog inserts an audit_log row. Errors are swallowed (best-effort audit).
// Uses pool directly (not tx) so audit rows persist even when the outer tx rolls back.
func (s *WebhookService) auditLog(ctx context.Context, ev auditEvent) {
	metaJSON, _ := json.Marshal(ev.Metadata)
	_, _ = s.pool.Exec(ctx,
		`INSERT INTO audit_log (user_id, event, metadata) VALUES ($1, $2, $3)`,
		ev.UserID, ev.Event, metaJSON,
	)
}
