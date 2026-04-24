// transaction_service.go — Phase 05: pending topup intent creation + cancel.
// [F2] 12-hex order code from UUID[:6] + idempotency via idx_tx_user_pkg_pending.
// [Q2] Cancel → status='cancelled' (not 'failed') to preserve provider_ref for late-payment recovery.
package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrUnknownPackage is returned when a package code is not in the Packages map.
var ErrUnknownPackage = errors.New("unknown_package")

// ErrProviderRefCollisionMaxRetries is returned when 3 consecutive UUID-derived
// order codes all collide on idx_tx_provider_ref_active (astronomically rare).
var ErrProviderRefCollisionMaxRetries = errors.New("provider_ref collision after 3 retries")

// ErrBankConfigMissing returned when SEPAY_BANK_CODE or SEPAY_BANK_ACCOUNT env is empty.
// Surfaces early at /buy confirm time rather than producing a broken QR URL.
var ErrBankConfigMissing = errors.New("sepay bank config missing (SEPAY_BANK_CODE and/or SEPAY_BANK_ACCOUNT env required)")

// TransactionService manages pending topup intents and their lifecycle.
type TransactionService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	rdb  *goredis.Client
	cfg  *config.Config
	log  *zap.Logger
}

// NewTransactionService constructs a TransactionService. All fields required.
func NewTransactionService(
	pool *pgxpool.Pool,
	q *sqlcdb.Queries,
	rdb *goredis.Client,
	cfg *config.Config,
	log *zap.Logger,
) *TransactionService {
	return &TransactionService{pool: pool, q: q, rdb: rdb, cfg: cfg, log: log}
}

// CreateTopupIntent creates (or returns an existing) pending transaction for userID+pkgCode.
// Idempotent via idx_tx_user_pkg_pending; retries up to 3x on idx_tx_provider_ref_active.
// Returns (transaction row, QR URL, nil) on success.
func (s *TransactionService) CreateTopupIntent(
	ctx context.Context,
	userID uuid.UUID,
	pkgCode string,
) (sqlcdb.Transaction, string, error) {
	// Step 1: validate package code.
	if err := ValidatePackageCode(pkgCode); err != nil {
		return sqlcdb.Transaction{}, "", ErrUnknownPackage
	}
	pkg := Packages[pkgCode]

	// [H2] Fail-closed on missing bank config — producing a QR with empty acc= would
	// yield a technically-valid URL that SePay rejects, leaving orphaned pending rows.
	// Better to surface the misconfig early at /buy confirm than silently later.
	if s.cfg.SepayBankAccount == "" || s.cfg.SepayBankCode == "" {
		return sqlcdb.Transaction{}, "", ErrBankConfigMissing
	}

	// Step 2: Redis lock (UX fast path debounce — non-fatal).
	if s.rdb != nil {
		lockKey := fmt.Sprintf("topup_lock:%s:%s", userID, pkgCode)
		if err := s.rdb.SetNX(ctx, lockKey, "1", 30*time.Second).Err(); err != nil {
			s.log.Warn("CreateTopupIntent: redis SetNX failed (non-fatal)",
				zap.String("user_id", userID.String()),
				zap.String("pkg", pkgCode),
				zap.Error(err),
			)
		}
	}

	// Step 3: retry loop for provider_ref collision.
	for attempt := 0; attempt < 3; attempt++ {
		u := uuid.New()
		// [F2] 12 uppercase hex chars from first 6 bytes of UUID (48-bit entropy).
		orderCode := strings.ToUpper(hex.EncodeToString(u[:6]))

		ref := orderCode // string copy for *string param
		tx, err := s.q.InsertPendingTransaction(ctx, sqlcdb.InsertPendingTransactionParams{
			UserID:          userID,
			ProviderRef:     &ref,
			PackageCode:     pkgCode,
			AmountVnd:       pkg.AmountVND,
			PremiumGranted:  int32(pkg.PremiumCredits),  //nolint:gosec // credits fit int32
			StandardGranted: int32(pkg.StandardCredits), //nolint:gosec
		})
		if err == nil {
			qrURL := sepay.BuildQRURL(
				s.cfg.SepayBankCode,
				s.cfg.SepayBankAccount,
				pkg.AmountVND,
				fmt.Sprintf("SBF TOPUP %s", orderCode),
			)
			return tx, qrURL, nil
		}

		// Inspect Postgres constraint name for idempotency / retry branch.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "idx_tx_user_pkg_pending":
				// User already has a pending row for this package — return it (idempotent UX).
				existing, qErr := s.q.GetPendingTxByUserPackage(ctx, sqlcdb.GetPendingTxByUserPackageParams{
					UserID:      userID,
					PackageCode: pkgCode,
				})
				if qErr != nil {
					return sqlcdb.Transaction{}, "", fmt.Errorf("CreateTopupIntent: fetch existing pending: %w", qErr)
				}
				existingRef := ""
				if existing.ProviderRef != nil {
					existingRef = *existing.ProviderRef
				}
				qrURL := sepay.BuildQRURL(
					s.cfg.SepayBankCode,
					s.cfg.SepayBankAccount,
					existing.AmountVnd,
					fmt.Sprintf("SBF TOPUP %s", existingRef),
				)
				return existing, qrURL, nil

			case "idx_tx_provider_ref_active":
				// Extremely rare birthday collision — retry with fresh UUID.
				s.log.Warn("CreateTopupIntent: provider_ref collision — retrying",
					zap.String("order_code", orderCode),
					zap.Int("attempt", attempt),
				)
				continue
			}
		}
		return sqlcdb.Transaction{}, "", fmt.Errorf("CreateTopupIntent: insert: %w", err)
	}
	return sqlcdb.Transaction{}, "", ErrProviderRefCollisionMaxRetries
}

// CancelPendingTransaction flips status to 'cancelled' (NOT 'failed') per [Q2].
// Preserves provider_ref in idx_tx_provider_ref_active so Phase 06 webhook can
// flip to 'recovered_by_late_payment' if SePay delivers a late payment.
// reason is stored in metadata.cancel_reason for audit trail.
func (s *TransactionService) CancelPendingTransaction(
	ctx context.Context,
	txID uuid.UUID,
	reason string,
) error {
	err := s.q.CancelPendingTransaction(ctx, sqlcdb.CancelPendingTransactionParams{
		ID:      txID,
		Column2: reason,
	})
	if err != nil {
		return fmt.Errorf("CancelPendingTransaction: %w", err)
	}
	return nil
}

// GetPendingForUserPackage returns (tx, true, nil) if a pending row exists for
// the given user+package. Returns (zero, false, nil) when none found.
func (s *TransactionService) GetPendingForUserPackage(
	ctx context.Context,
	userID uuid.UUID,
	pkgCode string,
) (sqlcdb.Transaction, bool, error) {
	tx, err := s.q.GetPendingTxByUserPackage(ctx, sqlcdb.GetPendingTxByUserPackageParams{
		UserID:      userID,
		PackageCode: pkgCode,
	})
	if err != nil {
		// pgx returns pgx.ErrNoRows when no row matches — treat as "not found".
		if isNoRows(err) {
			return sqlcdb.Transaction{}, false, nil
		}
		return sqlcdb.Transaction{}, false, fmt.Errorf("GetPendingForUserPackage: %w", err)
	}
	return tx, true, nil
}

// GetByProviderRef retrieves a transaction by its SePay order code.
// Used by the Phase 06 webhook to locate the pending row on payment arrival.
func (s *TransactionService) GetByProviderRef(
	ctx context.Context,
	ref string,
) (sqlcdb.Transaction, error) {
	tx, err := s.q.GetTxByProviderRef(ctx, &ref)
	if err != nil {
		return sqlcdb.Transaction{}, fmt.Errorf("GetByProviderRef: %w", err)
	}
	return tx, nil
}

// isNoRows returns true for pgx "no rows" sentinel errors.
func isNoRows(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}
