package service

import (
	"context"

	"github.com/google/uuid"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/verification"
	"go.uber.org/zap"
)

// VerificationService is a thin facade over the verification.Verifier.
// It exposes only the methods needed by API handlers and background runners,
// keeping the handler layer decoupled from the internal verification package.
type VerificationService struct {
	v   *verification.Verifier
	log *zap.Logger
}

// NewVerificationService constructs a VerificationService.
// q is passed directly to the verifier; log may be nil (nop logger used).
func NewVerificationService(q *sqlcdb.Queries, log *zap.Logger) *VerificationService {
	if log == nil {
		log = zap.NewNop()
	}
	return &VerificationService{
		v:   verification.New(q, log),
		log: log.Named("verification_svc"),
	}
}

// Start runs the verifier's background loop. Designed to be called as a goroutine.
func (s *VerificationService) Start(ctx context.Context) {
	s.v.Start(ctx)
}

// Verify enqueues a job ID for async post-publish verification.
// Non-blocking: if the internal channel is full, the ticker will retry.
func (s *VerificationService) Verify(jobID uuid.UUID) {
	s.v.Verify(jobID)
}
