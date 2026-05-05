package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ApiHandlerDeps holds all service dependencies injected into v1 HTTP handlers.
// All pointer fields are nil-safe: handlers must guard with if dep == nil checks.
type ApiHandlerDeps struct {
	Pool    *pgxpool.Pool
	Rdb     *goredis.Client
	Queries *sqlcdb.Queries

	KeySvc      *service.KeyService
	UserSvc     *service.UserService
	WalletSvc   *service.WalletService
	TxSvc       *service.TransactionService
	AuditSvc    *service.AuditService
	WpSiteSvc   *service.WpSiteService
	CampaignSvc *service.CampaignService
	JobSvc      *service.JobService
	ContentSvc  *service.ContentService
	ExecutionSvc *service.ExecutionService

	// WorkerSharedToken authenticates the legacy external worker plane (WORKER_SHARED_TOKEN).
	WorkerSharedToken string
	// WorkerEnabled reflects cfg.WorkerEnabled; used by GET /health/worker.
	WorkerEnabled bool
	// WorkerID is the embedded worker's identity string (hostname-pid); empty when disabled.
	WorkerID string

	Log *zap.Logger
}
