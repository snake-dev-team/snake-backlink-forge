package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ApiHandlerDeps struct {
	Pool      *pgxpool.Pool
	Rdb       *goredis.Client
	Queries   *sqlcdb.Queries
	KeySvc    *service.KeyService
	UserSvc   *service.UserService
	WalletSvc *service.WalletService
	TxSvc     *service.TransactionService
	AuditSvc  *service.AuditService
	WpSiteSvc *service.WpSiteService
	Log       *zap.Logger
}
