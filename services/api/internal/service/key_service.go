// key_service.go — Phase 03 KeyService: issue, revoke, mask display.
// Satisfies the KeyIssuer interface declared in user_service.go by method signature.
//
// Security invariants:
//   - Plaintext key is NEVER logged, stored, or returned after the initial Issue call.
//   - Only key_prefix (first 12 chars) and key_hash (SHA-256) are persisted.
//   - Audit log insert is best-effort: failure does not abort Issue.
//
// Concurrency invariant (§1.3):
//   - DB partial UNIQUE index idx_keys_user_active_unique enforces at most 1 active key
//     per user at the storage layer (migration 20260425001).
//   - Issue catches pgconn error 23505 (unique_violation) on concurrent race and retries
//     once. If still conflicted after maxRetries, returns ErrKeyRaceContention.
package service

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
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/util"
	"go.uber.org/zap"
)

var (
	// ErrKeyRaceContention is returned when concurrent Issue calls for the same user
	// both reach the insert step and the retry budget is exhausted.
	// Callers should surface this as a transient "system busy" error to the user.
	ErrKeyRaceContention = errors.New("key issue race contention — max retries exceeded")
	ErrInvalidKey        = errors.New("invalid api key")
)

// KeyService issues and manages API keys.
// Implements KeyIssuer (Issue method) — bot.Deps.KeyService accepts *KeyService directly
// because bot handlers also call GetActiveMasked which is not on the interface.
type KeyService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	log  *zap.Logger
}

// NewKeyService constructs a KeyService.
func NewKeyService(pool *pgxpool.Pool, log *zap.Logger) *KeyService {
	return &KeyService{
		pool: pool,
		q:    sqlcdb.New(pool),
		log:  log,
	}
}

// Issue revokes any existing active key for userID and inserts a new one atomically.
// Returns (plaintext, prefix, nil) on success.
// Plaintext must be shown to the user ONCE and then discarded — it is not retrievable.
//
// Concurrency safety: if a concurrent Issue wins the partial UNIQUE index race,
// this call retries once. On second conflict it returns ErrKeyRaceContention.
// Audit log fires only on FINAL successful commit, not per attempt.
func (s *KeyService) Issue(ctx context.Context, userID uuid.UUID) (plaintext string, prefix string, err error) {
	const maxRetries = 1 // 1 retry after first 23505 conflict
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var newKeyID uuid.UUID
		plaintext, prefix, newKeyID, err = s.issueOnce(ctx, userID)
		if err == nil {
			// Post-commit: structured log with prefix ONLY — plaintext never logged.
			s.log.Info("api key issued",
				zap.String("user_id", userID.String()),
				zap.String("key_prefix", prefix),
			)
			// Best-effort audit log — fires only on final success.
			go s.insertAuditLog(context.Background(), userID, newKeyID, prefix)
			return plaintext, prefix, nil
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "40001") {
			// 23505 = unique_violation: concurrent insert beat us to the active slot.
			// 40001 = serialization_failure: Serializable tx detected a phantom write conflict.
			// Both are recoverable: retry so THIS caller gets their own fresh plaintext.
			// Contract: caller always receives their own plaintext on success.
			if attempt < maxRetries {
				s.log.Warn("key issue conflict — retrying",
					zap.String("user_id", userID.String()),
					zap.String("pg_code", pgErr.Code),
					zap.Int("attempt", attempt+1),
				)
				continue
			}
			return "", "", ErrKeyRaceContention
		}
		return "", "", err
	}
	return "", "", ErrKeyRaceContention
}

// issueOnce is a single attempt of the revoke→insert transaction.
// Returns (plaintext, prefix, newKeyID, error). 23505 propagates to Issue's retry loop.
func (s *KeyService) issueOnce(ctx context.Context, userID uuid.UUID) (string, string, uuid.UUID, error) {
	plaintext, hashBytes, prefix, err := util.GenerateAPIKey()
	if err != nil {
		return "", "", uuid.Nil, fmt.Errorf("key_service.issueOnce: generate: %w", err)
	}

	// Serializable isolation prevents phantom insert race: two concurrent transactions
	// that both pass the "no active row exists" check would otherwise both insert,
	// violating §1.3. Under Serializable, one will receive a serialization failure
	// (SQLSTATE 40001) which we treat equivalently to 23505 — both trigger a retry.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return "", "", uuid.Nil, fmt.Errorf("key_service.issueOnce: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := sqlcdb.New(tx)

	// Revoke any existing active key (0 or 1 rows — enforced by single-active invariant).
	if revokeErr := qtx.RevokeActiveKeysForUser(ctx, userID); revokeErr != nil {
		return "", "", uuid.Nil, fmt.Errorf("key_service.issueOnce: revoke old keys: %w", revokeErr)
	}

	// Insert new key row — returns full row including generated UUID.
	name := "primary"
	newKey, insertErr := qtx.InsertKey(ctx, sqlcdb.InsertKeyParams{
		UserID:    userID,
		KeyHash:   hashBytes,
		KeyPrefix: prefix,
		Name:      &name,
	})
	if insertErr != nil {
		return "", "", uuid.Nil, fmt.Errorf("key_service.issueOnce: insert key: %w", insertErr)
	}

	if err = tx.Commit(ctx); err != nil {
		return "", "", uuid.Nil, fmt.Errorf("key_service.issueOnce: commit: %w", err)
	}

	return plaintext, prefix, newKey.ID, nil
}

func (s *KeyService) ValidatePlaintext(ctx context.Context, plaintext string) (uuid.UUID, bool, string, error) {
	if !strings.HasPrefix(plaintext, "sbf_live_") || len(plaintext) < 16 {
		return uuid.Nil, false, "", ErrInvalidKey
	}

	sum := sha256.Sum256([]byte(plaintext))
	row, err := s.q.GetKeyByHash(ctx, sum[:])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, "", ErrInvalidKey
		}
		return uuid.Nil, false, "", fmt.Errorf("key_service.ValidatePlaintext: key lookup: %w", err)
	}

	user, err := s.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return uuid.Nil, false, "", fmt.Errorf("key_service.ValidatePlaintext: user lookup: %w", err)
	}

	return row.UserID, user.IsBanned, row.KeyPrefix, nil
}

// GetActiveMasked returns a display-safe masked representation of the user's active key.
// Format: "<prefix>•••••<last4hexofhash>" e.g. "sbf_live_Zk3•••••dVo5".
// Returns ("", false, nil) when no active key exists for the user.
func (s *KeyService) GetActiveMasked(ctx context.Context, userID uuid.UUID) (maskedDisplay string, exists bool, err error) {
	row, err := s.q.GetActiveKeyByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("key_service.GetActiveMasked: query: %w", err)
	}

	// Build masked display: prefix (12) + bullet chars + last 4 hex chars of hash[:4].
	// We take first 4 bytes of hash → 8 hex chars → last 4 = suffix for display.
	hashHex := hex.EncodeToString(row.KeyHash[:4]) // 8 hex chars from first 4 bytes
	suffix := hashHex[len(hashHex)-4:]             // last 4 hex chars

	masked := row.KeyPrefix + "•••••" + suffix
	return masked, true, nil
}

// insertAuditLog inserts a best-effort audit record for a key_issued event.
// Called in a goroutine after commit; errors are logged but do not propagate.
// keyID populates audit_log.key_id so events are traceable to the specific key row.
func (s *KeyService) insertAuditLog(ctx context.Context, userID uuid.UUID, keyID uuid.UUID, keyPrefix string) {
	meta, _ := json.Marshal(map[string]string{"key_prefix": keyPrefix})
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (user_id, key_id, event, metadata) VALUES ($1, $2, $3, $4)`,
		userID, keyID, "key_issued", meta,
	)
	if err != nil {
		s.log.Warn("key_service: audit log insert failed",
			zap.String("user_id", userID.String()),
			zap.String("key_prefix", keyPrefix),
			zap.Error(err),
		)
	}
}
