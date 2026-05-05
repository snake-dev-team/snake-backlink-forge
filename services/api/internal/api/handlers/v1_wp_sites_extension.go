package handlers

// v1_wp_sites_extension.go — Extension-only handler for fetching WP site credentials.
// Route: GET /api/v1/wp-sites/by-domain/:domain
// Protected by: AuthAPIKey + ExtensionSignature middleware (signed request required).
// Security: returns plaintext app_password — do NOT log response body; audit every call.

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// V1WPSiteByDomainExt returns decrypted WP site credentials for the requesting user's
// site matching the given domain. Extension-signed routes only.
//
// Response 200: { base_url, app_username, app_password_plain }
// Response 404: no connected wp_site for user + domain
// Response 400: missing domain param
// Response 401: not authenticated
// Response 503: service unavailable (WpSiteSvc nil)
func V1WPSiteByDomainExt(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.WpSiteSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "wp_sites_unavailable"})
		}

		domain := c.Params("domain")
		if domain == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "detail": "domain required"})
		}

		// Audit every credential fetch — sensitive endpoint.
		if deps.AuditSvc != nil {
			_, _ = deps.AuditSvc.Log(c.Context(), service.AuditInput{
				UserID: &apiUser.ID,
				Event:  "wp_password_fetched",
				Metadata: map[string]any{
					"domain":     domain,
					"key_prefix": apiUser.KeyPrefix,
				},
			})
		}

		creds, err := deps.WpSiteSvc.GetByDomainPlain(c.Context(), apiUser.ID, domain)
		if errors.Is(err, service.ErrWPSiteNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found"})
		}
		if err != nil {
			if deps.Log != nil {
				deps.Log.Warn("wp_site GetByDomainPlain failed", zap.Error(err), zap.String("domain", domain))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}

		// Return plaintext creds — extension uses these to authenticate against WP REST API.
		// SECURITY: response is NOT logged by Fiber's logger (body logging disabled globally).
		return c.JSON(creds)
	}
}
