// webhook_service_process.go — Phase 06: processTransaction implementation (split from webhook_service.go).
// Contains the CAS UPDATE, [F1] 3-way branch, base grants, and commit logic.
//
// H1 fix: audit + admin alert for sepay_overpaid fire AFTER tx.Commit (not inside handleOverpaid).
// H2 fix: base grants (premium→standard) run BEFORE bonus grant. Spec lines 49-51.
// H3 fix: bonus computed once inside handleOverpaid, returned to processTransaction. No duplicate calc.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"go.uber.org/zap"
)

// DeadLetterSentinel is the reserved provider_ref value that forces ErrDeadLetterSentinel.
// Used by retry consumer integration tests — never appears in real transactions.
const DeadLetterSentinel = "DEADLETTER_TRIGGER"

// txRow holds the columns returned by the CAS UPDATE query.
type txRow struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Premium   int
	Standard  int
	AmountVND int64
	PkgCode   string
	PreStatus string // 'pending' or 'cancelled' — used to detect Q2 recovery
}

// overpaidResult carries the pre-commit data computed by handleOverpaidPreCommit.
// audit + alert are emitted AFTER commit succeeds to avoid ghost rows on commit failure.
type overpaidResult struct {
	bonus     int   // bonus credits granted (0 if rate==0 or diff==0)
	diff      int64 // positive excess VND
	pool      string
	alertable bool // diff >= 10_000 — send admin alert post-commit
}

// processTransaction executes the full CAS-gated payment flow inside a ReadCommitted txn.
func (s *WebhookService) processTransaction(
	ctx context.Context,
	orderCode string,
	p sepay.Payload,
) (ProcessResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ProcessResult{}, fmt.Errorf("processTransaction: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	payloadJSON, _ := json.Marshal(map[string]any{"sepay_raw": p})

	// [Q2] CAS: widen gate to ('pending','cancelled'); capture pre_status for branch logic.
	// FOR UPDATE inside CTE serialises concurrent webhooks for the same provider_ref.
	var row txRow
	err = tx.QueryRow(ctx, `
		WITH target AS (
			SELECT id, status AS pre_status
			FROM transactions
			WHERE provider_ref = $1 AND status IN ('pending','cancelled')
			FOR UPDATE
		)
		UPDATE transactions t
		SET
			status      = CASE target.pre_status
			                WHEN 'pending' THEN 'paid'::transaction_status
			                ELSE 'recovered_by_late_payment'::transaction_status
			              END,
			paid_at     = NOW(),
			metadata    = t.metadata || $2::jsonb,
			updated_at  = NOW()
		FROM target
		WHERE t.id = target.id
		RETURNING t.id, t.user_id, t.premium_granted, t.standard_granted,
		          t.amount_vnd, t.package_code, target.pre_status`,
		orderCode, payloadJSON,
	).Scan(&row.ID, &row.UserID, &row.Premium, &row.Standard,
		&row.AmountVND, &row.PkgCode, &row.PreStatus)

	if errors.Is(err, pgx.ErrNoRows) {
		return s.handleNoRowsCase(ctx, orderCode)
	}
	if err != nil {
		return ProcessResult{}, fmt.Errorf("processTransaction: CAS UPDATE: %w", err)
	}

	wasCancelled := row.PreStatus == "cancelled"

	// [F1] 3-way amount branch.
	diff := p.TransferAmount - row.AmountVND
	switch {
	case diff < 0:
		return s.handleUnderpaid(ctx, tx, row, diff, orderCode, wasCancelled)
	case diff == 0:
		// Exact match — fall through to base grants below.
	case diff > 0:
		// [H2] Set manual_review flag inside tx (DB write) — this is correct pre-commit.
		// Bonus Grant happens AFTER base grants to satisfy spec ledger row order (lines 49-51).
		// Bonus and alert/audit deferred to after commit (H1+H3).
		if err := s.handleOverpaidPreCommit(ctx, tx, row, diff); err != nil {
			return ProcessResult{}, err
		}
	}

	// [H2] Base grants FIRST (spec steps 5-6): premium → standard.
	if err := s.applyBaseGrants(ctx, tx, row); err != nil {
		return ProcessResult{}, err
	}

	// [H2+H3] Bonus grant AFTER base grants (spec step 7).
	// Compute bonus once here — single source of truth.
	var overpaid *overpaidResult
	if diff > 0 {
		pool, rate := perCreditRate(row.Premium, row.Standard, row.AmountVND)
		bonus := 0
		if rate > 0 {
			bonus = int(diff / rate) // floor division
		}
		overpaid = &overpaidResult{
			bonus:     bonus,
			diff:      diff,
			pool:      pool,
			alertable: diff >= 10_000,
		}
		// Grant bonus credits (topup_excess) — inside tx, after base grants.
		if bonus > 0 {
			if _, gErr := s.wallet.Grant(ctx, tx, GrantInput{
				UserID:    row.UserID,
				Pool:      pool,
				Amount:    bonus,
				EventType: "topup_excess",
				RefType:   "transaction",
				RefID:     row.ID,
			}); gErr != nil {
				return ProcessResult{}, fmt.Errorf("processTransaction: bonus grant: %w", gErr)
			}
		}
	}

	// AddVNDSpent: use transferAmount (actual paid), not amount_vnd (package nominal).
	if _, err := tx.Exec(ctx,
		`UPDATE wallets SET total_vnd_spent = total_vnd_spent + $2 WHERE user_id = $1`,
		row.UserID, p.TransferAmount,
	); err != nil {
		return ProcessResult{}, fmt.Errorf("processTransaction: AddVNDSpent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ProcessResult{}, fmt.Errorf("processTransaction: commit: %w", err)
	}

	// [H1] Post-commit side-effects only — audit + admin alert fire here, never before Commit.
	// If commit failed above, we returned early; no ghost audit rows or false alerts.
	bonus := 0
	if overpaid != nil {
		bonus = overpaid.bonus
		s.auditLog(ctx, auditEvent{
			Event:  "sepay_overpaid",
			UserID: &row.UserID,
			Metadata: map[string]any{
				"order_code":    orderCode,
				"diff":          overpaid.diff,
				"bonus_credits": overpaid.bonus,
				"transfer_amt":  p.TransferAmount,
			},
		})
		if overpaid.alertable && s.adminAlertCh != nil {
			s.log.Info("overpaid: sending admin alert",
				zap.String("order_code", orderCode),
				zap.Int64("excess_vnd", overpaid.diff),
				zap.Int("bonus_credits", overpaid.bonus),
			)
			select {
			case s.adminAlertCh <- notify.AdminAlert{
				Kind:         "overpaid",
				OrderCode:    orderCode,
				UserID:       row.UserID.String(),
				ExcessVND:    overpaid.diff,
				BonusCredits: overpaid.bonus,
			}:
			default:
				s.log.Warn("admin alert channel full, dropping overpaid alert",
					zap.String("order_code", orderCode),
				)
			}
		}
	}

	s.auditLog(ctx, auditEvent{
		Event:  "sepay_success",
		UserID: &row.UserID,
		Metadata: map[string]any{
			"order_code":    orderCode,
			"transfer_amt":  p.TransferAmount,
			"was_cancelled": wasCancelled,
			"overpaid":      diff > 0,
			"bonus_credits": bonus,
		},
	})

	return ProcessResult{
		UserID:       row.UserID,
		PackageCode:  row.PkgCode,
		Premium:      row.Premium,
		Standard:     row.Standard,
		WasCancelled: wasCancelled,
		Overpaid:     diff > 0,
		BonusCredits: bonus,
	}, nil
}

// handleNoRowsCase runs an out-of-band status check to distinguish AlreadyProcessed from UnknownOrder.
func (s *WebhookService) handleNoRowsCase(ctx context.Context, orderCode string) (ProcessResult, error) {
	var st string
	qErr := s.pool.QueryRow(ctx,
		`SELECT status FROM transactions WHERE provider_ref = $1 LIMIT 1`, orderCode,
	).Scan(&st)

	if errors.Is(qErr, pgx.ErrNoRows) {
		s.auditLog(ctx, auditEvent{
			Event:    "sepay_unknown_order",
			Metadata: map[string]any{"order_code": orderCode},
		})
		return ProcessResult{UnknownOrder: true}, nil
	}
	if qErr != nil {
		return ProcessResult{}, fmt.Errorf("handleNoRowsCase: status lookup: %w", qErr)
	}

	// paid / recovered_by_late_payment → idempotent replay (normal case).
	// manual_review / failed / refunded → treat as already-handled terminal state.
	s.auditLog(ctx, auditEvent{
		Event:    "sepay_replay",
		Metadata: map[string]any{"order_code": orderCode, "status": st},
	})
	return ProcessResult{AlreadyProcessed: true}, nil
}

// handleUnderpaid sets status='manual_review', commits, audits, and returns Underpaid=true.
// No credits are granted on underpayment.
func (s *WebhookService) handleUnderpaid(
	ctx context.Context,
	tx pgx.Tx,
	row txRow,
	diff int64, // negative: transferAmount - amountVND
	orderCode string,
	wasCancelled bool,
) (ProcessResult, error) {
	underpaidDiff := -diff // positive magnitude
	if _, err := tx.Exec(ctx, `
		UPDATE transactions
		SET status   = 'manual_review'::transaction_status,
		    metadata = metadata || jsonb_build_object('underpaid_diff', $2::bigint)
		WHERE id = $1`, row.ID, underpaidDiff,
	); err != nil {
		return ProcessResult{}, fmt.Errorf("handleUnderpaid: update: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ProcessResult{}, fmt.Errorf("handleUnderpaid: commit: %w", err)
	}
	// Audit fires after commit — consistent with H1 pattern.
	s.auditLog(ctx, auditEvent{
		Event:  "sepay_underpaid",
		UserID: &row.UserID,
		Metadata: map[string]any{
			"order_code":     orderCode,
			"underpaid_diff": underpaidDiff,
		},
	})
	return ProcessResult{Underpaid: true, UserID: row.UserID, WasCancelled: wasCancelled}, nil
}

// handleOverpaidPreCommit performs only the DB writes that must happen inside the transaction:
// flagging manual_review when diff >= 50k. It does NOT grant bonus credits (done post-base-grants),
// does NOT emit audit rows, and does NOT send admin alerts — all deferred to post-commit (H1).
func (s *WebhookService) handleOverpaidPreCommit(
	ctx context.Context,
	tx pgx.Tx,
	row txRow,
	diff int64, // positive: transferAmount - amountVND
) error {
	// diff >= 50k → flag for manual review (credits still granted).
	if diff >= 50_000 {
		if _, err := tx.Exec(ctx, `
			UPDATE transactions
			SET metadata = metadata || jsonb_build_object(
				'manual_review', true,
				'overpaid_diff', $2::bigint
			)
			WHERE id = $1`, row.ID, diff,
		); err != nil {
			return fmt.Errorf("handleOverpaidPreCommit: manual_review flag: %w", err)
		}
	}
	return nil
}

// applyBaseGrants calls wallet.Grant for premium and/or standard credits from the tx row snapshot.
func (s *WebhookService) applyBaseGrants(ctx context.Context, tx pgx.Tx, row txRow) error {
	if row.Premium > 0 {
		if _, err := s.wallet.Grant(ctx, tx, GrantInput{
			UserID:    row.UserID,
			Pool:      "premium",
			Amount:    row.Premium,
			EventType: "topup",
			RefType:   "transaction",
			RefID:     row.ID,
		}); err != nil {
			return fmt.Errorf("applyBaseGrants: premium: %w", err)
		}
	}
	if row.Standard > 0 {
		if _, err := s.wallet.Grant(ctx, tx, GrantInput{
			UserID:    row.UserID,
			Pool:      "standard",
			Amount:    row.Standard,
			EventType: "topup",
			RefType:   "transaction",
			RefID:     row.ID,
		}); err != nil {
			return fmt.Errorf("applyBaseGrants: standard: %w", err)
		}
	}
	return nil
}
