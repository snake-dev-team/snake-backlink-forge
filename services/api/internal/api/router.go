// Package api wires together the Fiber application, middleware, and route handlers.
package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api/handlers"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Register mounts all API routes onto app.
// Both pool and rdb may be nil in Phase 1 (lenient boot).
func Register(app *fiber.App, pool *pgxpool.Pool, rdb *goredis.Client) {
	app.Get("/health", handlers.Health)
	app.Get("/ready", handlers.Ready(pool, rdb))
}

func RegisterV1(app *fiber.App, deps *handlers.ApiHandlerDeps) {
	v1 := app.Group("/api/v1")
	v1.Get("/health", handlers.Health)
	v1.Options("/*", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	v1.Post("/auth/verify", middleware.NewAuthVerifyRateLimit(deps.Rdb, deps.Log), handlers.V1AuthVerify(deps))

	authed := v1.Group("", middleware.AuthAPIKey(deps.KeySvc, deps.AuditSvc, deps.Log))
	authed.Get("/me", handlers.V1Me(deps))
	authed.Get("/me/usage", handlers.V1Usage(deps))
	authed.Get("/balance", handlers.V1Balance(deps))
	authed.Get("/transactions", handlers.V1Transactions(deps))
	authed.Get("/ledger", handlers.V1Ledger(deps))
	authed.Get("/wp-sites", handlers.V1WpSitesList(deps))
	authed.Post("/wp-sites", handlers.V1WpSitesCreate(deps))
	authed.Delete("/wp-sites/:id", handlers.V1WpSitesDelete(deps))
	authed.Post("/wp-sites/:id/revalidate", handlers.V1WpSitesRevalidate(deps))

	authed.Get("/campaigns", handlers.V1CampaignsList(deps))
	authed.Post("/campaigns", handlers.V1CampaignsCreate(deps))
	authed.Get("/campaigns/:id", handlers.V1CampaignGet(deps))
	authed.Post("/campaigns/:id/start", handlers.V1CampaignStatus(deps, sqlcdb.CampaignStatusRunning))
	authed.Post("/campaigns/:id/pause", handlers.V1CampaignStatus(deps, sqlcdb.CampaignStatusPaused))
	authed.Post("/campaigns/:id/archive", handlers.V1CampaignStatus(deps, sqlcdb.CampaignStatusArchived))
	authed.Post("/campaigns/:id/enqueue", handlers.V1CampaignEnqueue(deps))
	authed.Get("/campaigns/:id/jobs", handlers.V1CampaignJobs(deps))
	// Pass Redis client so nonce replay protection is multi-instance safe (F4).
	// When deps.Rdb is nil (dev/test without Redis), falls back to in-process cache.
	extensionAuthed := authed.Group("", middleware.ExtensionSignature(deps.Rdb))
	extensionAuthed.Get("/campaign/next", handlers.V1CampaignNext(deps))
	extensionAuthed.Post("/campaign/result/:id", handlers.V1CampaignResult(deps))
	// Signed extension-only: returns decrypted WP App Password for job execution.
	// Audit-logged on every call; response body must not be captured in access logs.
	extensionAuthed.Get("/wp-sites/by-domain/:domain", handlers.V1WPSiteByDomainExt(deps))
	authed.Get("/targets", handlers.V1TargetsList(deps))
	authed.Post("/targets", handlers.V1TargetsCreate(deps))
}

// RegisterWebhook mounts the SePay webhook route with its middleware chain.
// Called from server.go after WebhookDeps are fully wired in main.go.
//
// Middleware order (outermost first):
//  1. NewRateLimitWebhook — [Q4] 20 req/sec/IP via Redis fixed-window; runs BEFORE auth.
//  2. SePayWebhook handler — auth, parse, CAS grant, notify goroutine.
//
// Body size cap: Fiber app-level BodyLimit (64KB) set in server.go covers this route.
func RegisterWebhook(app *fiber.App, deps *handlers.WebhookDeps, log *zap.Logger) {
	app.Post("/webhooks/sepay",
		middleware.NewRateLimitWebhook(deps.Rdb, 20, log),
		handlers.SePayWebhook(deps),
	)
}
