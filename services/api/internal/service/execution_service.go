package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

var ErrExecutionUnavailable = errors.New("execution_unavailable")

type ExecutionService struct {
	pool       *pgxpool.Pool
	q          *sqlcdb.Queries
	leaseTTL   time.Duration
	retryLimit int
	model      string
	log        *zap.Logger
}

type ExecutionLease struct {
	JobID             uuid.UUID
	UserID            uuid.UUID
	CampaignID        uuid.UUID
	TargetID          *uuid.UUID // nullable: NULL for campaign_target_sites (wp_sites) path
	TargetURLSnapshot string
	AnchorText        string
	AnchorType        string
	ContentBody       *string
	ContentTitle      *string
	ContentMeta       *string
	MoneySiteURL      string
	Pool              string
	Model             string
	LeaseExpiresAt    time.Time
}

func NewExecutionService(pool *pgxpool.Pool, q *sqlcdb.Queries, leaseSec, retryLimit int, model string, log *zap.Logger) *ExecutionService {
	if leaseSec <= 0 {
		leaseSec = 300
	}
	if retryLimit <= 0 {
		retryLimit = 3
	}
	return &ExecutionService{pool: pool, q: q, leaseTTL: time.Duration(leaseSec) * time.Second, retryLimit: retryLimit, model: model, log: log}
}

func (s *ExecutionService) LeaseNext(ctx context.Context) (ExecutionLease, error) {
	if s == nil || s.pool == nil {
		return ExecutionLease{}, ErrExecutionUnavailable
	}
	leaseUntil := time.Now().UTC().Add(s.leaseTTL)
	row := s.pool.QueryRow(ctx, `
WITH picked AS (
    SELECT j.id
    FROM jobs j
    JOIN campaigns c ON c.id = j.campaign_id
    WHERE c.status = 'running'
      AND (
        j.status IN ('content_ready', 'queued')
        OR (j.status = 'dispatched' AND j.dispatched_at < NOW() - ($1::int * INTERVAL '1 second'))
      )
    ORDER BY CASE WHEN j.status = 'content_ready' THEN 0 WHEN j.status = 'queued' THEN 1 ELSE 2 END, j.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
), claimed AS (
    UPDATE jobs j
    SET status = 'dispatched', dispatched_at = NOW()
    FROM picked
    WHERE j.id = picked.id
    RETURNING j.id, j.user_id, j.campaign_id, j.target_id, j.target_url_snapshot,
              j.anchor_text, j.anchor_type, j.content_body, j.content_title, j.content_meta,
              j.pool
)
SELECT claimed.id, claimed.user_id, claimed.campaign_id, claimed.target_id,
       claimed.target_url_snapshot, claimed.anchor_text, claimed.anchor_type,
       claimed.content_body, claimed.content_title, claimed.content_meta,
       claimed.pool, c.money_site_url
FROM claimed
JOIN campaigns c ON c.id = claimed.campaign_id`, int32(s.leaseTTL/time.Second))

	lease := ExecutionLease{Model: s.model, LeaseExpiresAt: leaseUntil}
	if err := row.Scan(&lease.JobID, &lease.UserID, &lease.CampaignID, &lease.TargetID, &lease.TargetURLSnapshot, &lease.AnchorText, &lease.AnchorType, &lease.ContentBody, &lease.ContentTitle, &lease.ContentMeta, &lease.Pool, &lease.MoneySiteURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExecutionLease{}, ErrJobUnavailable
		}
		return ExecutionLease{}, fmt.Errorf("execution lease: %w", err)
	}
	return lease, nil
}

// Report records a terminal job outcome from the legacy external worker plane.
// The modern embedded worker reports via JobService.Report directly. This thin
// implementation just flips status + result fields; credit/cooldown bookkeeping
// is intentionally NOT mirrored here — legacy callers must be migrated.
func (s *ExecutionService) Report(ctx context.Context, jobID uuid.UUID, in JobResultInput) error {
	if s == nil || s.pool == nil || s.q == nil {
		return ErrExecutionUnavailable
	}
	status, ok := normalizeReportStatus(in.Status)
	if !ok {
		return ErrJobInvalid
	}
	cmd, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = $1::job_status,
		    result_url = $2,
		    error_code = $3,
		    error_message = $4,
		    evidence = $5,
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $6
		  AND status IN ('dispatched', 'in_progress', 'queued', 'content_ready')`,
		status, in.ResultURL, in.ErrorCode, in.ErrorMessage, in.Evidence, jobID,
	)
	if err != nil {
		return fmt.Errorf("execution report: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrJobNotFound
	}
	return nil
}
