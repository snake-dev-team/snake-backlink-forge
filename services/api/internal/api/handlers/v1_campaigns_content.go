package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/ai"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"go.uber.org/zap"
)

const generateContentTimeout = 15 * time.Minute

type generateContentRequest struct {
	TonePreference string `json:"tone_preference"`
}

// V1GenerateCampaignContent handles POST /api/v1/campaigns/:id/generate-content.
// Returns 202 immediately and runs generation in a background goroutine.
// The goroutine is bounded by a 15-minute context; progress is written to the DB
// incrementally (each job updated independently), so partial completion is durable.
func V1GenerateCampaignContent(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		if deps.ContentSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "content_generation_unavailable",
			})
		}

		campaignID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}

		var body generateContentRequest
		// Body is optional — ignore parse error; defaults apply in service.
		_ = c.BodyParser(&body)

		// Fetch queued job count + campaign validity before accepting the request.
		// This surfaces not-found early (synchronously) before the 202 is sent.
		campaign, err := deps.CampaignSvc.Get(c.Context(), apiUser.ID, campaignID)
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}

		// Count queued jobs needing content to return job_count in 202 body.
		queuedCount, err := deps.ContentSvc.CountQueuedJobsNeedingContent(c.Context(), apiUser.ID, campaignID)
		if err != nil {
			logAPIWarn(deps, "content: count queued jobs failed", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}

		// Estimate cost: assume ~3K tokens per article at claude-sonnet-4-6 pricing.
		// $3/M input (~1K tokens) + $15/M output (~2K tokens) ≈ $0.033/article.
		const estimatedCostPerJob = 0.033
		estimatedCost := float64(queuedCount) * estimatedCostPerJob

		// Spawn background generation; capture values needed by goroutine.
		userID := apiUser.ID
		tone := body.TonePreference
		log := deps.Log

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), generateContentTimeout)
			defer cancel()

			result, genErr := deps.ContentSvc.GenerateForCampaign(ctx, userID, campaignID, tone)
			if genErr != nil {
				if errors.Is(genErr, ai.ErrAIUnavailable) {
					log.Warn("content generation: AI unavailable",
						zap.String("campaign_id", campaignID.String()),
					)
				} else {
					log.Error("content generation: background task failed",
						zap.String("campaign_id", campaignID.String()),
						zap.String("campaign_name", campaign.Name),
						zap.Error(genErr),
					)
				}
				return
			}
			log.Info("content generation: completed",
				zap.String("campaign_id", campaignID.String()),
				zap.Int("jobs_processed", result.JobsProcessed),
				zap.Float64("cost_usd", result.EstimatedCostUSD),
			)
		}()

		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"job_count":          queuedCount,
			"estimated_cost_usd": estimatedCost,
		})
	}
}

