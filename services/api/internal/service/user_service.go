// Package service contains domain services for Snake Backlink Forge.
// UserService handles user lifecycle: upsert, phone verification, and trial grant.
//
// Import graph (no cycles):
//   bot → service (for UserService + KeyIssuer interface)
//   service → db/sqlc, util (phone)
//   service does NOT import bot
//
// Phase 03 KeyService lives in this same package and satisfies KeyIssuer without
// any circular dependency — both service files share the `service` package.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ─────────────────────────── Sentinel errors ───────────────────────────────

var (
	// ErrTrialAlreadyUsed means the user row already has trial_used=TRUE.
	ErrTrialAlreadyUsed = errors.New("trial_already_used")
	// ErrTrialPhoneReused means another user already claimed trial with this phone.
	// Raised by SQLSTATE 23505 on idx_users_phone_trial (partial unique index).
	ErrTrialPhoneReused = errors.New("trial_phone_reused")
	// ErrTrialUserBanned means the user is banned and cannot use the trial.
	ErrTrialUserBanned = errors.New("trial_user_banned")
	// ErrKeyIssuerNotWired is returned by noopKeyIssuer until Phase 03 wires real KeyService.
	ErrKeyIssuerNotWired = errors.New("key issuer not wired (phase 03 pending)")
)

// ─────────────────────────── KeyIssuer interface ───────────────────────────

// KeyIssuer is the minimal interface UserService needs from Phase 03 KeyService.
// Declared here (in service package) so that:
//   - bot package imports service only (no circular import risk)
//   - Phase 03 KeyService (also in service package) satisfies this naturally
//   - noopKeyIssuer below is the placeholder until Phase 03 wires the real impl
type KeyIssuer interface {
	Issue(ctx context.Context, userID uuid.UUID) (plaintext string, prefix string, err error)
}

// noopKeyIssuer is wired at startup until Phase 03 provides the real KeyService.
// UserService.VerifyContactAndGrantTrial returns ErrKeyIssuerNotWired on key issuance;
// the bot handler catches this and replies without showing a plaintext key.
type noopKeyIssuer struct{}

func (noopKeyIssuer) Issue(_ context.Context, _ uuid.UUID) (string, string, error) {
	return "", "", ErrKeyIssuerNotWired
}

// NoopKeyIssuer is the exported zero-value placeholder for use in main.go and tests.
// Replace with real KeyService in Phase 03.
var NoopKeyIssuer KeyIssuer = noopKeyIssuer{}

// ─────────────────────────── UserService ───────────────────────────────────

// UserService handles user lifecycle: upsert stub, phone verify, trial grant.
type UserService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	keys KeyIssuer
	log  *zap.Logger
	// rdb is reserved for Phase 04 session/cache needs; held here to avoid
	// changing the constructor signature later. Not used in Phase 02.
	rdb *goredis.Client //nolint:unused
}

// New constructs a UserService. keys may be NoopKeyIssuer until Phase 03.
func New(pool *pgxpool.Pool, rdb *goredis.Client, keys KeyIssuer, log *zap.Logger) *UserService {
	if keys == nil {
		keys = NoopKeyIssuer
	}
	return &UserService{
		pool: pool,
		q:    sqlcdb.New(pool),
		keys: keys,
		rdb:  rdb,
		log:  log,
	}
}

// EnsureStub upserts the Telegram user and guarantees a wallet row exists.
// Runs both operations in a single transaction to keep them atomic.
// Called by loadUser middleware on every bot update.
func (s *UserService) EnsureStub(ctx context.Context, tgID int64, tgUsername, firstName string) (sqlcdb.User, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return sqlcdb.User{}, fmt.Errorf("user_service.EnsureStub: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := sqlcdb.New(tx)

	// Build nullable username.
	var usernamePtr *string
	if tgUsername != "" {
		usernamePtr = &tgUsername
	} else if firstName != "" {
		// Store firstName as fallback telegram_username display if real username absent.
		usernamePtr = &firstName
	}

	user, err := qtx.UpsertUserStub(ctx, sqlcdb.UpsertUserStubParams{
		TelegramID:       tgID,
		TelegramUsername: usernamePtr,
	})
	if err != nil {
		return sqlcdb.User{}, fmt.Errorf("user_service.EnsureStub: upsert: %w", err)
	}

	if err := qtx.EnsureWallet(ctx, user.ID); err != nil {
		return sqlcdb.User{}, fmt.Errorf("user_service.EnsureStub: ensure wallet: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.User{}, fmt.Errorf("user_service.EnsureStub: commit: %w", err)
	}

	return user, nil
}
