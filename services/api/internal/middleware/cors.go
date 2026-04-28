package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func NewCORS(allowedOrigins []string) fiber.Handler {
	if len(allowedOrigins) == 0 {
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(allowedOrigins, ","),
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Authorization,Content-Type",
		MaxAge:           600,
	})
}
