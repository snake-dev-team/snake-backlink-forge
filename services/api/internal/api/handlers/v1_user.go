package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"golang.org/x/sync/errgroup"
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

// V1Usage returns aggregate usage statistics for the current user.
// Sites: total connected wp_sites (not soft-deleted).
// CreditsConsumedMonth: positive int representing credits consumed since
// start of current calendar month in Asia/Ho_Chi_Minh timezone (VN-only user base).
// CampaignsRunning: campaigns where status='running'.
//
// Three independent COUNT queries run in parallel via errgroup
// (~10ms total instead of ~30ms serialized).
//
// Security: GET endpoint protected by sameSite:strict cookie + Bearer token
// middleware. Origin guard correctly skips GET (origin.ts:27-29).
func V1Usage(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.Queries == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "api_unavailable"})
		}

		ctx := c.Context()
		var sites, credits, camps int64
		g, gctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			var err error
			sites, err = deps.Queries.CountWpSitesByUser(gctx, apiUser.ID)
			return err
		})
		g.Go(func() error {
			var err error
			credits, err = deps.Queries.CountCreditsConsumedThisMonth(gctx, apiUser.ID)
			return err
		})
		g.Go(func() error {
			var err error
			camps, err = deps.Queries.CountRunningCampaignsByUser(gctx, apiUser.ID)
			return err
		})

		if err := g.Wait(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}

		return c.JSON(fiber.Map{
			"sites_connected":        sites,
			"credits_consumed_month": credits,
			"campaigns_running":      camps,
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
