// admin_service.go — Phase 08: admin-only operations (stats, grant, ban, unban).
// Lookup logic lives in admin_service_lookup.go.
// Every write action (grant, ban, unban) produces an audit_log row within the same pgx.Tx.
package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// ─────────────────────────── Sentinel errors ───────────────────────────────

var (
	// ErrCannotBanAdmin is returned when admin tries to ban another admin (M3 self-ban guard).
	ErrCannotBanAdmin = errors.New("cannot ban admin user")
	// ErrAdminGrantInvalidPool is returned for pool values outside {premium, standard}.
	ErrAdminGrantInvalidPool = errors.New("invalid pool")
	// ErrAdminGrantAmountOOB is returned when amount is not in [1, 10000].
	ErrAdminGrantAmountOOB = errors.New("amount out of bounds (1..10000)")
	// ErrAdminUserNotFound is returned when the target user does not exist.
	ErrAdminUserNotFound = errors.New("user not found")
)

// AdminStats aggregates dashboard numbers for /admin stats reply.
type AdminStats struct {
	Users           int64
	UsersVerified   int64
	UsersBanned     int64
	UsersTrialUsed  int64
	ActiveKeys      int64
	PaidTx24h       int64
	Revenue24h      int64
	PendingTx       int64
	ManualReviewTx  int64
	OutstandingPremium  int64
	OutstandingStandard int64
	OpenTicketsCount    int64
	LastAuthFails       []sqlcdb.AuditLog
}

// AdminService handles admin-only operations: stats, grant, ban, unban, lookup.
type AdminService struct {
	pool  *pgxpool.Pool
	q     *sqlcdb.Queries
	audit *AuditService
	cfg   *config.Config
	log   *zap.Logger
}

// NewAdminService constructs an AdminService.
func NewAdminService(pool *pgxpool.Pool, q *sqlcdb.Queries, audit *AuditService, cfg *config.Config, log *zap.Logger) *AdminService {
	return &AdminService{pool: pool, q: q, audit: audit, cfg: cfg, log: log}
}

// Stats returns an AdminStats snapshot. All reads are independent queries (no tx).
// Acceptable for dashboard use — not a SERIALIZABLE snapshot.
func (s *AdminService) Stats(ctx context.Context) (AdminStats, error) {
	var st AdminStats
	var err error

	if st.Users, err = s.q.CountUsers(ctx); err != nil {
		return st, fmt.Errorf("admin_service.Stats: count users: %w", err)
	}
	if st.UsersVerified, err = s.q.CountUsersVerified(ctx); err != nil {
		return st, fmt.Errorf("admin_service.Stats: count verified: %w", err)
	}
	if st.UsersBanned, err = s.q.CountUsersBanned(ctx); err != nil {
		return st, fmt.Errorf("admin_service.Stats: count banned: %w", err)
	}
	if st.UsersTrialUsed, err = s.q.CountUsersTrialUsed(ctx); err != nil {
		return st, fmt.Errorf("admin_service.Stats: count trial: %w", err)
	}
	if st.ActiveKeys, err = s.q.CountActiveKeys(ctx); err != nil {
		return st, fmt.Errorf("admin_service.Stats: count keys: %w", err)
	}

	txRow, err := s.q.TxStats24h(ctx)
	if err != nil {
		return st, fmt.Errorf("admin_service.Stats: tx stats: %w", err)
	}
	st.PaidTx24h = txRow.Paid24h
	st.Revenue24h = txRow.Revenue24h
	st.PendingTx = txRow.Pending
	st.ManualReviewTx = txRow.ManualReview

	credRow, err := s.q.CreditsOutstanding(ctx)
	if err != nil {
		return st, fmt.Errorf("admin_service.Stats: credits: %w", err)
	}
	st.OutstandingPremium = credRow.Premium
	st.OutstandingStandard = credRow.Standard

	// Global open ticket count via raw query (no per-user filter needed).
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM support_tickets WHERE status IN ('open','in_progress')`,
	).Scan(&st.OpenTicketsCount); err != nil {
		return st, fmt.Errorf("admin_service.Stats: open tickets: %w", err)
	}

	// Last 10 auth_fail events in the last 24h.
	st.LastAuthFails, err = s.q.GetAuditLogByEventSince(ctx, sqlcdb.GetAuditLogByEventSinceParams{
		Event:     "sepay_auth_fail",
		CreatedAt: nowMinus24h(),
		Limit:     10,
	})
	if err != nil {
		return st, fmt.Errorf("admin_service.Stats: auth fails: %w", err)
	}

	return st, nil
}

// Grant — [F5 Option A] atomically grants credits to a user and records the audit entry.
// Single pgx.Tx: grant_credits → currval(ledger_id_seq) → INSERT audit_log → COMMIT.
// amount must be in [1, 10000]; pool must be "standard" or "premium".
func (s *AdminService) Grant(ctx context.Context, adminTGID int64, targetUserID uuid.UUID, pool string, amount int, reason string) (newBalance int, err error) {
	if !validPools[pool] {
		return 0, ErrAdminGrantInvalidPool
	}
	if amount < 1 || amount > 10000 {
		return 0, ErrAdminGrantAmountOOB
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, fmt.Errorf("admin_service.Grant: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Step 1: grant_credits with ref_type='user', ref_id=target_user_id (UUID fits UUID column).
	if err := tx.QueryRow(ctx,
		`SELECT grant_credits($1, $2, $3, 'admin_adjust', 'user', $1)`,
		targetUserID, pool, amount,
	).Scan(&newBalance); err != nil {
		return 0, fmt.Errorf("admin_service.Grant: grant_credits: %w", err)
	}

	// Step 2: capture ledger_id via currval — safe within same session per Postgres spec.
	var ledgerID int64
	if err := tx.QueryRow(ctx, `SELECT currval('ledger_id_seq')`).Scan(&ledgerID); err != nil {
		return 0, fmt.Errorf("admin_service.Grant: currval ledger_id: %w", err)
	}

	// Step 3: INSERT audit_log with ledger_id in metadata (bigint → jsonb).
	_, auditErr := s.audit.LogIntoTx(ctx, tx, AuditInput{
		UserID: &targetUserID,
		Event:  "admin_grant",
		Metadata: map[string]any{
			"ledger_id":    ledgerID,
			"amount":       amount,
			"pool":         pool,
			"admin_tg_id":  adminTGID,
			"reason":       reason,
			"source":       "telegram_admin",
		},
	})
	if auditErr != nil {
		return 0, fmt.Errorf("admin_service.Grant: audit insert: %w", auditErr)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("admin_service.Grant: commit: %w", err)
	}
	return newBalance, nil
}

// Ban — [M3] bans a user by telegram_id. Rejects if target is in the admin allowlist.
// UPDATE + audit_log in same tx for consistency.
func (s *AdminService) Ban(ctx context.Context, adminTGID, targetTGID int64, reason string) error {
	// [M3] Self-ban guard: reject if target tg_id ∈ cfg.AdminTelegramIDs.
	if slices.Contains(s.cfg.AdminTelegramIDs, targetTGID) {
		return ErrCannotBanAdmin
	}

	// Resolve user to get UUID for audit metadata.
	targetUser, err := s.q.GetUserByTelegramID(ctx, targetTGID)
	if err != nil {
		return fmt.Errorf("admin_service.Ban: resolve user: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("admin_service.Ban: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `UPDATE users SET is_banned = TRUE, updated_at = NOW() WHERE telegram_id = $1`, targetTGID); err != nil {
		return fmt.Errorf("admin_service.Ban: update: %w", err)
	}

	if _, err := s.audit.LogIntoTx(ctx, tx, AuditInput{
		UserID: &targetUser.ID,
		Event:  "admin_ban",
		Metadata: map[string]any{
			"admin_tg_id": adminTGID,
			"reason":      reason,
			"source":      "telegram_admin",
		},
	}); err != nil {
		return fmt.Errorf("admin_service.Ban: audit: %w", err)
	}

	return tx.Commit(ctx)
}

// GetUserByTGID resolves a Telegram ID to the user's UUID.
// Returns ErrAdminUserNotFound (wrapping pgx.ErrNoRows) when no match.
// Used by cmd_admin.go before calling Grant (which takes uuid.UUID).
func (s *AdminService) GetUserByTGID(ctx context.Context, tgID int64) (uuid.UUID, error) {
	u, err := s.q.GetUserByTelegramID(ctx, tgID)
	if err != nil {
		return uuid.UUID{}, ErrAdminUserNotFound
	}
	return u.ID, nil
}

// Unban lifts a ban by telegram_id, recording an audit_unban row.
func (s *AdminService) Unban(ctx context.Context, adminTGID, targetTGID int64, reason string) error {
	targetUser, err := s.q.GetUserByTelegramID(ctx, targetTGID)
	if err != nil {
		return fmt.Errorf("admin_service.Unban: resolve user: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("admin_service.Unban: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `UPDATE users SET is_banned = FALSE, updated_at = NOW() WHERE telegram_id = $1`, targetTGID); err != nil {
		return fmt.Errorf("admin_service.Unban: update: %w", err)
	}

	if _, err := s.audit.LogIntoTx(ctx, tx, AuditInput{
		UserID: &targetUser.ID,
		Event:  "admin_unban",
		Metadata: map[string]any{
			"admin_tg_id": adminTGID,
			"reason":      reason,
			"source":      "telegram_admin",
		},
	}); err != nil {
		return fmt.Errorf("admin_service.Unban: audit: %w", err)
	}

	return tx.Commit(ctx)
}
