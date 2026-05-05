// audit_service.go — Phase 08: thin wrapper around audit_log INSERT.
// Used exclusively by AdminService (admin write actions require explicit audit).
// Phase 02/03/06 services continue using inline pool.Exec for their audit calls.
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// AuditInput holds parameters for a single audit_log row insertion.
// Nil pointers for optional fields result in NULL columns (per schema).
type AuditInput struct {
	UserID   *uuid.UUID
	KeyID    *uuid.UUID
	Event    string
	IPHash   *string    // sha256(ip) hex; nil for bot/admin actions (no IP available)
	Country  *string
	Metadata map[string]any // serialized to jsonb
}

// AuditService wraps audit_log INSERT operations.
// Exported for use by AdminService. Phase 02/03/06 services do NOT use this —
// they issue inline pool.Exec to avoid coupling money-flow code to this service.
type AuditService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	log  *zap.Logger
}

// NewAuditService constructs an AuditService.
func NewAuditService(pool *pgxpool.Pool, q *sqlcdb.Queries, log *zap.Logger) *AuditService {
	return &AuditService{pool: pool, q: q, log: log}
}

// Log inserts an audit_log row via the pool (own connection, no transaction).
// Returns the inserted row's ID. Best-effort for non-critical callers;
// admin actions should propagate the error (write integrity requirement).
func (s *AuditService) Log(ctx context.Context, in AuditInput) (int64, error) {
	params, err := buildAuditParams(in)
	if err != nil {
		return 0, fmt.Errorf("audit_service.Log: build params: %w", err)
	}
	row, err := s.q.InsertAuditLog(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("audit_service.Log: insert: %w", err)
	}
	return row.ID, nil
}

// LogIntoTx inserts an audit_log row inside the caller-provided pgx.Tx.
// Required by AdminService.Grant for atomic audit-with-grant:
// grant fails → tx rollback → audit NOT inserted.
// audit INSERT fails → tx rollback → grant + ledger dropped.
func (s *AuditService) LogIntoTx(ctx context.Context, tx pgx.Tx, in AuditInput) (int64, error) {
	params, err := buildAuditParams(in)
	if err != nil {
		return 0, fmt.Errorf("audit_service.LogIntoTx: build params: %w", err)
	}
	qtx := sqlcdb.New(tx)
	row, err := qtx.InsertAuditLog(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("audit_service.LogIntoTx: insert: %w", err)
	}
	return row.ID, nil
}

// buildAuditParams converts AuditInput → sqlcdb.InsertAuditLogParams.
// Metadata map is JSON-marshaled; nil map → empty JSON object "{}".
func buildAuditParams(in AuditInput) (sqlcdb.InsertAuditLogParams, error) {
	meta := in.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return sqlcdb.InsertAuditLogParams{}, fmt.Errorf("marshal metadata: %w", err)
	}

	var ipHash []byte
	if in.IPHash != nil {
		ipHash = []byte(*in.IPHash)
	}

	return sqlcdb.InsertAuditLogParams{
		UserID:   in.UserID,
		KeyID:    in.KeyID,
		Event:    in.Event,
		IpHash:   ipHash,
		Country:  in.Country,
		Metadata: metaJSON,
	}, nil
}
