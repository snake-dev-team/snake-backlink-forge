// Package middleware holds Fiber middleware constructors for the API service.
package middleware

import (
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// NewLogger returns a Fiber handler that emits one structured log line per
// request using the provided zap.Logger (via fiberzap contrib adapter).
// Fields logged: ip, latency, status code, HTTP method, URL path.
func NewLogger(log *zap.Logger) fiber.Handler {
	return fiberzap.New(fiberzap.Config{
		Logger: log,
		Fields: []string{"ip", "latency", "status", "method", "url"},
	})
}
