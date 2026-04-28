package handlers

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

type authVerifyRequest struct {
	Key string `json:"key"`
}

func V1AuthVerify(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if deps.KeySvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "auth_unavailable"})
		}

		var body authVerifyRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}

		userID, isBanned, prefix, err := deps.KeySvc.ValidatePlaintext(c.Context(), body.Key)
		if err != nil {
			if !errors.Is(err, service.ErrInvalidKey) && deps.Log != nil {
				deps.Log.Warn("auth verify failed", zap.Error(err))
			}
			logWebLogin(deps, c, "auth.web.fail", nil, "")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_key"})
		}
		if isBanned {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "account_banned"})
		}

		logWebLogin(deps, c, "auth.web.login", &userID, prefix)
		return c.JSON(fiber.Map{"ok": true, "user": fiber.Map{"id": userID, "key_prefix": prefix}})
	}
}

func logWebLogin(deps *ApiHandlerDeps, c *fiber.Ctx, event string, userID *uuid.UUID, keyPrefix string) {
	if deps.AuditSvc == nil {
		return
	}

	_, _ = deps.AuditSvc.Log(context.Background(), service.AuditInput{
		UserID: userID,
		Event:  event,
		Metadata: map[string]any{
			"connection_ip":   middleware.ConnIPFromCtx(c),
			"x_forwarded_for": c.Get(fiber.HeaderXForwardedFor),
			"key_prefix":      keyPrefix,
		},
	})
}
