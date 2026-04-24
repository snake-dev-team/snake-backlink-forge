// Package bot wires the Telegram bot into the Snake Backlink Forge API process.
// All bot sub-packages depend on this Deps container; no reverse imports allowed.
package bot

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Deps holds every shared resource the bot layer needs.
// Fields added in later phases (user/key/wallet services) can be nil until wired.
type Deps struct {
	Pool *pgxpool.Pool
	Rdb  *goredis.Client
	Log  *zap.Logger
	Cfg  *config.Config
}
