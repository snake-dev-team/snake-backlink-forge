package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
)

func V1Me(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.Queries == nil || deps.WalletSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "api_unavailable"})
		}

		user, err := deps.Queries.GetUserByID(c.Context(), apiUser.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		wallet, err := deps.WalletSvc.GetBalance(c.Context(), apiUser.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}

		return c.JSON(fiber.Map{
			"user_id":           user.ID,
			"telegram_id":       user.TelegramID,
			"telegram_username": user.TelegramUsername,
			"language":          user.Language,
			"is_verified":       user.IsVerified,
			"balance_credits":   wallet.PremiumCredits + wallet.StandardCredits,
			"premium_credits":   wallet.PremiumCredits,
			"standard_credits":  wallet.StandardCredits,
			"key_prefix":        apiUser.KeyPrefix,
		})
	}
}

func V1Balance(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.WalletSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "api_unavailable"})
		}

		wallet, err := deps.WalletSvc.GetBalance(c.Context(), apiUser.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		return c.JSON(fiber.Map{
			"balance_credits":  wallet.PremiumCredits + wallet.StandardCredits,
			"premium_credits":  wallet.PremiumCredits,
			"standard_credits": wallet.StandardCredits,
		})
	}
}

func V1Transactions(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.Queries == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "api_unavailable"})
		}

		limit := parseBoundedInt(c.Query("limit"), 20, 1, 100)
		offset := parseBoundedInt(c.Query("offset"), 0, 0, 10000)
		rows, err := deps.Queries.GetTxByUserPage(c.Context(), sqlcdb.GetTxByUserPageParams{UserID: apiUser.ID, Limit: int32(limit), Offset: int32(offset)})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		return c.JSON(fiber.Map{"items": rows, "limit": limit, "offset": offset})
	}
}

func V1Ledger(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.Queries == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "api_unavailable"})
		}

		limit := parseBoundedInt(c.Query("limit"), 20, 1, 100)
		offset := parseBoundedInt(c.Query("offset"), 0, 0, 10000)
		rows, err := deps.Queries.GetLedgerPage(c.Context(), sqlcdb.GetLedgerPageParams{UserID: apiUser.ID, Limit: int32(limit), Offset: int32(offset)})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		return c.JSON(fiber.Map{"items": rows, "limit": limit, "offset": offset})
	}
}

func parseBoundedInt(raw string, fallback int, minValue int, maxValue int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minValue || value > maxValue {
		return fallback
	}
	return value
}
