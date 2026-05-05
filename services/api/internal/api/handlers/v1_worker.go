package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
)

type workerReportRequest struct {
	JobID        string `json:"job_id"`
	Status       string `json:"status"`
	ResultURL    string `json:"result_url"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Evidence     string `json:"evidence"`
}

func V1WorkerLease(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !validWorkerToken(c, deps) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.ExecutionSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "execution_unavailable"})
		}
		lease, err := deps.ExecutionSvc.LeaseNext(c.Context())
		if err != nil {
			if errors.Is(err, service.ErrJobUnavailable) {
				return c.Status(fiber.StatusNoContent).Send(nil)
			}
			logAPIWarn(deps, "worker lease failed", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		return c.JSON(fiber.Map{"item": fiber.Map{
			"job_id":              lease.JobID,
			"user_id":             lease.UserID,
			"campaign_id":         lease.CampaignID,
			"target_id":           lease.TargetID,
			"target_url_snapshot": lease.TargetURLSnapshot,
			"anchor_text":         lease.AnchorText,
			"anchor_type":         lease.AnchorType,
			"content_body":        lease.ContentBody,
			"content_title":       lease.ContentTitle,
			"content_meta":        lease.ContentMeta,
			"money_site_url":      lease.MoneySiteURL,
			"pool":                lease.Pool,
			"model":               lease.Model,
			"lease_expires_at":    lease.LeaseExpiresAt,
		}})
	}
}

func V1WorkerReport(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !validWorkerToken(c, deps) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.ExecutionSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "execution_unavailable"})
		}
		var body workerReportRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		jobID, err := uuid.Parse(body.JobID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		err = deps.ExecutionSvc.Report(c.Context(), jobID, service.JobResultInput{
			Status:       body.Status,
			ResultURL:    body.ResultURL,
			ErrorCode:    body.ErrorCode,
			ErrorMessage: body.ErrorMessage,
			Evidence:     body.Evidence,
		})
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func validWorkerToken(c *fiber.Ctx, deps *ApiHandlerDeps) bool {
	if deps == nil || deps.WorkerSharedToken == "" {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(c.Get("Authorization"), "Bearer "))
	if token == "" {
		token = strings.TrimSpace(c.Get("X-Worker-Token"))
	}
	return token == deps.WorkerSharedToken
}
