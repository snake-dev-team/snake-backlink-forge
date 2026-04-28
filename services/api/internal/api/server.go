// Package api wires together the Fiber application, middleware, and route handlers.
package api

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// New builds and returns a configured *fiber.App with middleware and routes
// already mounted. It does NOT call Listen — callers start the server.
//
// Both pool and rdb may be nil; the health handlers degrade gracefully.
func New(cfg *config.Config, log *zap.Logger, pool *pgxpool.Pool, rdb *goredis.Client) *fiber.App {
	app := fiber.New(fiber.Config{
		// Show startup banner in development; suppress in prod containers.
		DisableStartupMessage: cfg.IsProduction(),

		// [C1] Hard cap at 64KB — SePay payloads are <2KB; reject oversized bodies to
		// prevent memory pressure and DoS amplification. Spec §non-functional line 59.
		BodyLimit: 64 * 1024,

		// Generous but bounded timeouts to prevent resource exhaustion.
		ReadTimeout:             15 * time.Second,
		WriteTimeout:            15 * time.Second,
		IdleTimeout:             60 * time.Second,
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          cfg.TrustedProxyCIDRs,

		// Return structured JSON on unhandled errors.
		ErrorHandler: jsonErrorHandler,
	})

	// Middleware: recover before logger so panics are captured in the same request log.
	app.Use(middleware.NewRecover(log, cfg))
	app.Use(middleware.NewLogger(log))
	app.Use(middleware.NewCORS(cfg.CORSOrigins))

	// Routes.
	Register(app, pool, rdb)

	return app
}

// jsonErrorHandler converts unhandled fiber errors into a JSON response.
func jsonErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}
