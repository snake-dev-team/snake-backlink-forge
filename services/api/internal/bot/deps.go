// Package bot wires the Telegram bot into the Snake Backlink Forge API process.
// All bot sub-packages depend on this Deps container; no reverse imports allowed.
//
// Import safety:
//   bot → service (for UserService + KeyIssuer interface declared in service package)
//   service does NOT import bot — no circular dependency.
//   Phase 03 will set KeyService on Deps; until then KeyService field is unused.
package bot

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Deps holds every shared resource the bot layer needs.
// Fields added in later phases can be nil until wired.
type Deps struct {
	Pool *pgxpool.Pool
	Rdb  *goredis.Client
	Log  *zap.Logger
	Cfg  *config.Config

	// UserService handles user upsert, phone verify, and trial grant.
	// Wired in main.go Phase 02 startup. Nil until DB is available.
	UserService *service.UserService

	// KeyService satisfies service.KeyIssuer for post-trial key issuance.
	// Wired in Phase 03. Until then service.NoopKeyIssuer is used inside UserService.
	// Kept on Deps so Phase 03 can inject without changing UserService constructor.
	KeyService service.KeyIssuer
}
