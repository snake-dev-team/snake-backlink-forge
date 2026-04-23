// Package middleware holds Fiber middleware constructors for the API service.
package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"go.uber.org/zap"
)

// NewRecover returns a Fiber panic-recovery handler that logs panics via zap.
// Stack traces are included in non-production environments to aid debugging.
func NewRecover(log *zap.Logger, cfg *config.Config) fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: !cfg.IsProduction(),
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			log.Error("panic recovered",
				zap.Any("error", e),
				zap.String("method", c.Method()),
				zap.String("path", c.Path()),
			)
		},
	})
}
