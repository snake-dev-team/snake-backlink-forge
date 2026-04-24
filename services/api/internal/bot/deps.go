// Package bot wires the Telegram bot into the Snake Backlink Forge API process.
// All bot sub-packages depend on this Deps container; no reverse imports allowed.
//
// Import safety:
//   bot → service (for UserService + KeyService declared in service package)
//   service does NOT import bot — no circular dependency.
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

	// KeyService handles API key issuance, revocation, and masked display.
	// Typed as *service.KeyService (not interface) because bot handlers call
	// GetActiveMasked in addition to Issue (which is on the KeyIssuer interface).
	// Phase 03: wired in main.go alongside UserService.
	KeyService *service.KeyService

	// WalletService handles credit grant/consume and balance reads.
	// Phase 04: wired in main.go. Nil until DB is available.
	WalletService *service.WalletService

	// TxService manages pending topup intents, QR generation, and cancel lifecycle.
	// Phase 05: wired in main.go. Nil until DB is available.
	TxService *service.TransactionService
}
