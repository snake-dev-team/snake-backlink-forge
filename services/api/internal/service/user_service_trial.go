package service

// user_service_trial.go — [F4] atomic trial gate implementation.
// Split from user_service.go to keep files under 200 lines.
// See phase-02-user-service.md §Architecture for full flow description.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/util"
	"go.uber.org/zap"
)

// VerifyContactAndGrantTrial is the [F4] atomic trial gate.
//
// Flow (all in one pgx.Tx, ReadCommitted):
//  1. SELECT trial_used, is_banned FROM users WHERE id=$1 FOR UPDATE  → row lock
//  2. if banned  → ROLLBACK, ErrTrialUserBanned
//  3. UPDATE users SET phone_e164=$1, is_verified=TRUE, trial_used=TRUE, updated_at=NOW()
//     WHERE id=$2 AND trial_used=FALSE
//     → SQLSTATE 23505 on idx_users_phone_trial  → ROLLBACK, ErrTrialPhoneReused
//     → RowsAffected == 0                         → ROLLBACK, ErrTrialAlreadyUsed
//  4. INSERT audit_log
//  5. SELECT grant_credits($2, 'standard', 5, 'trial_grant', 'user', $2)
//  6. COMMIT
//  7. (post-commit) keys.Issue(ctx, userID) — key table is not financial state
//
// The function returns (plaintext, prefix, nil) on success.
// plaintext is empty when keys == noopKeyIssuer (ErrKeyIssuerNotWired is swallowed
// and logged — bot handler checks for empty plaintext and responds accordingly).
func (s *UserService) VerifyContactAndGrantTrial(
	ctx context.Context,
	userID uuid.UUID,
	rawPhone string,
) (plaintext string, prefix string, err error) {
	phone, err := util.NormalizePhone(rawPhone)
	if err != nil {
		return "", "", fmt.Errorf("phone normalize: %w", err)
	}

	phoneHash := phoneHashPrefix(phone)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", "", fmt.Errorf("user_service.trial: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Step 1: Row lock — serializes concurrent /start taps for the same user.
	var trialUsed, isBanned bool
	err = tx.QueryRow(ctx,
		`SELECT trial_used, is_banned FROM users WHERE id=$1 FOR UPDATE`,
		userID,
	).Scan(&trialUsed, &isBanned)
	if err != nil {
		return "", "", fmt.Errorf("user_service.trial: lock row: %w", err)
	}

	// Step 2: Banned check.
	if isBanned {
		return "", "", ErrTrialUserBanned
	}

	// Step 3: Conditional UPDATE — partial index catches same-phone across different tg_ids.
	ct, err := tx.Exec(ctx,
		`UPDATE users
		 SET phone_e164=$1, is_verified=TRUE, trial_used=TRUE, updated_at=NOW()
		 WHERE id=$2 AND trial_used=FALSE`,
		phone, userID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			(pgErr.ConstraintName == "idx_users_phone_trial" ||
				strings.Contains(pgErr.Message, "phone")) {
			s.log.Info("trial gate: phone reused",
				zap.String("user_id", userID.String()),
				zap.String("phone_hash", phoneHash),
			)
			return "", "", ErrTrialPhoneReused
		}
		return "", "", fmt.Errorf("user_service.trial: update users: %w", err)
	}
	if ct.RowsAffected() == 0 {
		// trial_used was already TRUE — race between SELECT and UPDATE (rare).
		return "", "", ErrTrialAlreadyUsed
	}

	// Step 4: Audit log — event='trial_granted', phone_hash for security correlation.
	meta, _ := json.Marshal(map[string]string{
		"source":     "telegram_bot_start",
		"phone_hash": phoneHash,
	})
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_log (user_id, event, metadata) VALUES ($1, $2, $3)`,
		userID, "trial_granted", meta,
	)
	if err != nil {
		// Non-fatal: audit failure should not block trial grant.
		s.log.Warn("user_service.trial: audit log insert failed",
			zap.String("user_id", userID.String()), zap.Error(err))
	}

	// Step 5: Grant credits inside the same txn.
	var newBal int
	if err := tx.QueryRow(ctx,
		`SELECT grant_credits($1, 'standard', 5, 'trial_grant', 'user', $1)`,
		userID,
	).Scan(&newBal); err != nil {
		return "", "", fmt.Errorf("user_service.trial: grant_credits: %w", err)
	}

	// Step 6: Commit — all financial + phone state changes are now durable.
	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("user_service.trial: commit: %w", err)
	}

	s.log.Info("trial granted",
		zap.String("user_id", userID.String()),
		zap.String("phone_hash", phoneHash),
		zap.Int("balance_after", newBal),
	)

	// Step 7: Post-commit key issuance — key table is NOT financial state.
	// If noopKeyIssuer is wired, plaintext="" + prefix="" are returned to caller.
	// Bot handler (cmd_start.go) checks for empty plaintext and adjusts its reply.
	plaintext, prefix, keyErr := s.keys.Issue(ctx, userID)
	if keyErr != nil {
		if !isErrKeyIssuerNotWired(keyErr) {
			s.log.Warn("user_service.trial: key issue failed (trial grant committed, key missing)",
				zap.String("user_id", userID.String()), zap.Error(keyErr))
		}
		// Swallow: trial grant is committed; key can be issued on next /key command.
		return "", "", nil
	}

	return plaintext, prefix, nil
}

// phoneHashPrefix returns the first 8 hex chars of sha256(phone).
// Never log full phone numbers — PII.
func phoneHashPrefix(phone string) string {
	sum := sha256.Sum256([]byte(phone))
	return hex.EncodeToString(sum[:])[:8]
}

// isErrKeyIssuerNotWired checks whether the error is the noop sentinel.
func isErrKeyIssuerNotWired(err error) bool {
	return errors.Is(err, ErrKeyIssuerNotWired)
}
