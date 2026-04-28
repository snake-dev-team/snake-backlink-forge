package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

type ApiUser struct {
	ID        uuid.UUID
	IsBanned  bool
	KeyPrefix string
}

const ctxKeyApiUser = "api_user"

func AuthAPIKey(keySvc *service.KeyService, audit *service.AuditService, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}

		auth := c.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(auth, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if keySvc == nil {
			if log != nil {
				log.Warn("api key auth unavailable: key service nil")
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		userID, isBanned, prefix, err := keySvc.ValidatePlaintext(c.Context(), strings.TrimPrefix(auth, "Bearer "))
		if err != nil {
			if !errors.Is(err, service.ErrInvalidKey) && log != nil {
				log.Warn("api key validation failed", zap.Error(err))
			}
			logWebAuthFailure(audit, c, prefix)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if isBanned {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "account_banned"})
		}

		c.Locals(ctxKeyApiUser, ApiUser{ID: userID, IsBanned: isBanned, KeyPrefix: prefix})
		return c.Next()
	}
}

func ApiUserFromCtx(c *fiber.Ctx) (ApiUser, bool) {
	u, ok := c.Locals(ctxKeyApiUser).(ApiUser)
	return u, ok
}

func ConnIPFromCtx(c *fiber.Ctx) string {
	if c.Context() != nil && c.Context().RemoteAddr() != nil {
		return c.Context().RemoteAddr().String()
	}
	return ""
}

func logWebAuthFailure(audit *service.AuditService, c *fiber.Ctx, keyPrefix string) {
	if audit == nil {
		return
	}
	_, _ = audit.Log(context.Background(), service.AuditInput{
		Event: "auth.web.fail",
		Metadata: map[string]any{
			"connection_ip":   ConnIPFromCtx(c),
			"x_forwarded_for": c.Get(fiber.HeaderXForwardedFor),
			"key_prefix":      keyPrefix,
		},
	})
}
