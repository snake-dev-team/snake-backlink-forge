// referral_service.go — Phase 07: referral code generation and referral attribution.
// EnsureCode: idempotent get-or-create with 23505 collision retry.
// ProcessReferralOnStart: write-once attribution via WHERE referred_by IS NULL guard.
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/util"
	"go.uber.org/zap"
)

// ReferralService manages referral codes and attribution.
type ReferralService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	log  *zap.Logger
}

// NewReferralService constructs a ReferralService.
func NewReferralService(pool *pgxpool.Pool, q *sqlcdb.Queries, log *zap.Logger) *ReferralService {
	return &ReferralService{pool: pool, q: q, log: log}
}

// EnsureCode returns the user's referral code, creating one if none exists.
// - Generates a 6-char base58-uppercase code.
// - Retries up to 3x on unique-constraint violation (23505) before falling
//   back to an 8-char code for persistent collision resistance.
// - Idempotent: subsequent calls for the same user return the existing code.
func (s *ReferralService) EnsureCode(ctx context.Context, userID uuid.UUID) (string, error) {
	// Fast path: existing code.
	existing, err := s.q.GetReferralByUser(ctx, userID)
	if err == nil {
		return existing.Code, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	// Slow path: generate + insert with collision retry.
	// [H2 Opus review] Distinguish constraint by name — referrals table has TWO unique
	// constraints (user_id_key + code_key). 23505 on user_id means another goroutine
	// already created a row for this user (concurrent /ref) → re-fetch instead of retry.
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		code := util.GenBase58Upper(6)
		_, insertErr := s.q.InsertReferral(ctx, sqlcdb.InsertReferralParams{
			UserID: userID,
			Code:   code,
		})
		if insertErr == nil {
			s.log.Info("ReferralService.EnsureCode: created",
				zap.String("user_id", userID.String()),
				zap.String("code", code))
			return code, nil
		}
		if pgErr := asPgError(insertErr); pgErr != nil && pgErr.Code == "23505" {
			// Race-loser path: another concurrent EnsureCode for same user already inserted.
			// Re-fetch and return that row's code (idempotent contract).
			if isUserIDUniqueViolation(pgErr) {
				existing, fetchErr := s.q.GetReferralByUser(ctx, userID)
				if fetchErr == nil {
					s.log.Debug("ReferralService.EnsureCode: lost race, returning existing",
						zap.String("user_id", userID.String()), zap.String("code", existing.Code))
					return existing.Code, nil
				}
				return "", fetchErr
			}
			// Code collision — retry with fresh code.
			s.log.Debug("ReferralService.EnsureCode: code collision, retrying",
				zap.Int("attempt", i+1), zap.String("code", code))
			continue
		}
		return "", insertErr
	}

	// Fallback: 8-char code after 3 collisions (statistically near-impossible at current scale).
	code := util.GenBase58Upper(8)
	_, err = s.q.InsertReferral(ctx, sqlcdb.InsertReferralParams{
		UserID: userID,
		Code:   code,
	})
	if err != nil {
		return "", err
	}
	s.log.Warn("ReferralService.EnsureCode: used 8-char fallback code",
		zap.String("user_id", userID.String()))
	return code, nil
}

// GetTotalReferred returns the total_referred count for a user's referral row.
// Returns 0 if no referral row exists yet (before EnsureCode is called).
func (s *ReferralService) GetTotalReferred(ctx context.Context, userID uuid.UUID) (int32, error) {
	ref, err := s.q.GetReferralByUser(ctx, userID)
	if err != nil {
		return 0, nil // no row = 0 referred, not an error for display purposes
	}
	return ref.TotalReferred, nil
}

// ProcessReferralOnStart links a new user to a referrer identified by refCode.
// Idempotent via WHERE referred_by IS NULL guard — subsequent calls for the same
// newUserID are no-ops. Silent skip on invalid/own code.
func (s *ReferralService) ProcessReferralOnStart(ctx context.Context, newUserID uuid.UUID, refCode string) error {
	if refCode == "" {
		return nil
	}

	// Lookup referrer by code.
	referral, err := s.q.GetReferralByCode(ctx, refCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Invalid code — silent skip per spec.
			return nil
		}
		return err
	}

	// Own-code guard: user cannot refer themselves.
	if referral.UserID == newUserID {
		return nil
	}

	// Atomic: set referred_by + increment count in one transaction.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Write-once: only update if referred_by is NULL (idempotent guard).
	tag, err := tx.Exec(ctx,
		`UPDATE users SET referred_by = $1, updated_at = NOW() WHERE id = $2 AND referred_by IS NULL`,
		referral.UserID, newUserID,
	)
	if err != nil {
		return err
	}

	// If no rows updated, the user already has a referrer — silent no-op.
	if tag.RowsAffected() == 0 {
		return nil
	}

	// [H1 Opus review] Increment within the SAME tx — auto-commit pool path would
	// commit the count even if tx.Commit (referred_by UPDATE) later fails.
	if err := s.q.WithTx(tx).IncrementReferralCount(ctx, referral.UserID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.log.Info("ReferralService.ProcessReferralOnStart: attributed",
		zap.String("new_user_id", newUserID.String()),
		zap.String("referrer_id", referral.UserID.String()),
		zap.String("code", refCode))
	return nil
}

// asPgError extracts *pgconn.PgError from err if present, else returns nil.
// Reused from wallet_service.go pattern — same package, no duplication risk.
func asPgError(err error) *pgconn.PgError {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr
	}
	return nil
}

// isUserIDUniqueViolation returns true when a 23505 fired on referrals.user_id
// (rather than referrals.code) — distinguishes "race lost" from "code collision".
// Postgres surfaces ConstraintName when the driver supports it; fall back to
// substring match on the message ("user_id" appears in the auto-generated key
// name "referrals_user_id_key" and in the error detail).
func isUserIDUniqueViolation(pgErr *pgconn.PgError) bool {
	if pgErr.ConstraintName != "" {
		return pgErr.ConstraintName == "referrals_user_id_key"
	}
	// Fallback heuristic — Postgres detail typically includes the column name.
	return errors.Is(pgErr, pgx.ErrNoRows) == false &&
		(pgErr.Detail != "" && containsCI(pgErr.Detail, "user_id"))
}

func containsCI(s, sub string) bool {
	// Tiny case-insensitive contains — avoids importing strings here.
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
