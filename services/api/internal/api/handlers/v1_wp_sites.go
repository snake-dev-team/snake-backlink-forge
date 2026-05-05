package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

type wpSiteCreateRequest struct {
	BaseURL     string `json:"base_url"`
	AppUsername string `json:"app_username"`
	AppPassword string `json:"app_password"`
	Label       string `json:"label"`
}

func V1WpSitesList(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.WpSiteSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "wp_sites_unavailable"})
		}

		items, err := deps.WpSiteSvc.List(c.Context(), apiUser.ID)
		if err != nil {
			if deps.Log != nil {
				deps.Log.Warn("wp sites list failed", zap.Error(err))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		return c.JSON(fiber.Map{"items": wpSiteRows(items)})
	}
}

func V1WpSitesCreate(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.WpSiteSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "wp_sites_unavailable"})
		}

		var body wpSiteCreateRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		site, err := deps.WpSiteSvc.Create(c.Context(), apiUser.ID, service.WpSiteInput{
			BaseURL:     body.BaseURL,
			AppUsername: body.AppUsername,
			AppPassword: body.AppPassword,
			Label:       body.Label,
		})
		if err != nil {
			return writeWpSiteError(c, deps, err)
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": wpSite(site)})
	}
}

func V1WpSitesDelete(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.WpSiteSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "wp_sites_unavailable"})
		}

		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		if err := deps.WpSiteSvc.Delete(c.Context(), apiUser.ID, id); err != nil {
			if deps.Log != nil {
				deps.Log.Warn("wp site delete failed", zap.Error(err))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func V1WpSitesRevalidate(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.WpSiteSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "wp_sites_unavailable"})
		}

		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		status, err := deps.WpSiteSvc.Revalidate(c.Context(), apiUser.ID, id)
		if err != nil {
			if deps.Log != nil {
				deps.Log.Warn("wp site revalidate failed", zap.Error(err))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		return c.JSON(fiber.Map{"status": status})
	}
}

func writeWpSiteError(c *fiber.Ctx, deps *ApiHandlerDeps, err error) error {
	code := service.WpErrorCode(err)
	switch {
	case errors.Is(err, service.ErrWpInvalidURL):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": code})
	case errors.Is(err, service.ErrWpInvalidCredentials), errors.Is(err, service.ErrWpInsufficientCapability):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": code})
	case errors.Is(err, service.ErrWpRestAPINotFound):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": code})
	case errors.Is(err, service.ErrWpRateLimited):
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": code})
	case errors.Is(err, service.ErrWpServerError502), errors.Is(err, service.ErrWpServerError503), errors.Is(err, service.ErrWpServerError504):
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": code})
	case errors.Is(err, service.ErrWpPrivateAddress):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": code})
	default:
		if strings.Contains(err.Error(), "idx_wp_sites_user_url_active") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "wp_site_already_connected"})
		}
		if deps.Log != nil {
			deps.Log.Warn("wp site create failed", zap.Error(err))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
	}
}

func wpSiteRows(rows []sqlcdb.ListWpSitesByUserRow) []fiber.Map {
	items := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		items = append(items, fiber.Map{
			"id":                row.ID,
			"base_url":          row.BaseUrl,
			"app_username":      row.AppUsername,
			"label":             row.Label,
			"status":            row.Status,
			"last_validated_at": formatTimestamptz(row.LastValidatedAt),
			"last_error":        row.LastError,
			"created_at":        row.CreatedAt.Format(time.RFC3339),
			"updated_at":        row.UpdatedAt.Format(time.RFC3339),
		})
	}
	return items
}

func wpSite(row sqlcdb.WpSite) fiber.Map {
	return fiber.Map{
		"id":                row.ID,
		"base_url":          row.BaseUrl,
		"app_username":      row.AppUsername,
		"label":             row.Label,
		"status":            row.Status,
		"last_validated_at": formatTimestamptz(row.LastValidatedAt),
		"last_error":        row.LastError,
		"created_at":        row.CreatedAt.Format(time.RFC3339),
		"updated_at":        row.UpdatedAt.Format(time.RFC3339),
	}
}

func formatTimestamptz(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(time.RFC3339)
	return &formatted
}
