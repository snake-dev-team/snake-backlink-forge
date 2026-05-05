// Package bot wires the Telegram bot into the Snake Backlink Forge API process.
// All bot sub-packages depend on this Deps container; no reverse imports allowed.
//
// Import safety:
//   bot → service (for UserService + KeyService declared in service package)
//   service does NOT import bot — no circular dependency.
package bot

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/bot/templates"
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

	// SupportService creates support tickets with body cap + 3-ticket-per-user limit.
	// Phase 07: wired in main.go.
	SupportService *service.SupportService

	// RefService manages referral codes (6-char base58 upper, 23505 retry → 8-char fallback).
	// Phase 07.
	RefService *service.ReferralService

	// AdminService aggregates dashboard stats + atomic /admin grant (F5 Option A) + Ban/Unban/Lookup.
	// Phase 08.
	AdminService *service.AdminService

	// AuditService inserts audit_log rows. Used by AdminService (in-tx) and bot.AuditFailAlertWatcher.
	// Phase 08. Phase 02/03/06 use inline pool.Exec for audit (locked, not refactored).
	AuditService *service.AuditService

	// Templates renders user-facing message strings from the central registry
	// (templates package). All cmd_*.go handlers use Templates.Render(lang, key, data)
	// instead of inline strings. Phase 09.
	//
	// May be nil during early dev-mode boot — handlers that read it MUST treat
	// nil as "fall back to inline string" to avoid panicking before main.go
	// finishes wiring. The templateRender helper in cmd_helpers_template.go
	// implements this guard centrally.
	Templates *templates.Renderer

	// CampaignService handles campaign CRUD (create, list, get, set-status).
	// Phase 7.02: wired in main.go. Nil until DB is available.
	CampaignService *service.CampaignService

	// JobService handles job listing and KPI stats for campaign bot commands.
	// Phase 7.02: wired in main.go. Nil until DB is available.
	JobService *service.JobService
}
