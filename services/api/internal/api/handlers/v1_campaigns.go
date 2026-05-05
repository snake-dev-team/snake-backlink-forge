package handlers

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

type campaignCreateRequest struct {
	Name             string                    `json:"name"`
	MoneySiteURL     string                    `json:"money_site_url"`
	NicheKeywords    []string                  `json:"niche_keywords"`
	AnchorTexts      []service.AnchorTextInput `json:"anchor_texts"`
	Pool             string                    `json:"pool"`
	SourceMode       string                    `json:"source_mode"`
	DailyLimit       int32                     `json:"daily_limit"`
	EthicalMode      *bool                     `json:"ethical_mode"`
	NicheFilter      *bool                     `json:"niche_filter"`
	CreditsAllocated int32                     `json:"credits_allocated"`
	StartNow         bool                      `json:"start_now"`
}

type targetCreateRequest struct {
	URL       string   `json:"url"`
	Type      string   `json:"type"`
	Pool      string   `json:"pool"`
	Language  string   `json:"language"`
	NicheTags []string `json:"niche_tags"`
	Platform  string   `json:"platform"`
}

type enqueueRequest struct {
	Count int32 `json:"count"`
}

type jobReportRequest struct {
	Status       string `json:"status"`
	ResultURL    string `json:"result_url"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Evidence     string `json:"evidence"`
}

func V1CampaignsList(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.CampaignSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "campaigns_unavailable"})
		}
		items, err := deps.CampaignSvc.List(c.Context(), apiUser.ID, queryInt32(c, "limit", 25), queryInt32(c, "offset", 0))
		if err != nil {
			logAPIWarn(deps, "campaign list failed", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		}
		out := make([]fiber.Map, 0, len(items))
		for _, item := range items {
			out = append(out, campaignRow(item))
		}
		return c.JSON(fiber.Map{"items": out})
	}
}

func V1CampaignsCreate(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.CampaignSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "campaigns_unavailable"})
		}
		var body campaignCreateRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		ethicalMode, nicheFilter := true, true
		if body.EthicalMode != nil {
			ethicalMode = *body.EthicalMode
		}
		if body.NicheFilter != nil {
			nicheFilter = *body.NicheFilter
		}
		item, err := deps.CampaignSvc.Create(c.Context(), apiUser.ID, service.CampaignInput{
			Name:             body.Name,
			MoneySiteURL:     body.MoneySiteURL,
			NicheKeywords:    body.NicheKeywords,
			AnchorTexts:      body.AnchorTexts,
			Pool:             body.Pool,
			SourceMode:       body.SourceMode,
			DailyLimit:       body.DailyLimit,
			EthicalMode:      ethicalMode,
			NicheFilter:      nicheFilter,
			CreditsAllocated: body.CreditsAllocated,
			StartNow:         body.StartNow,
		})
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": campaign(item)})
	}
}

func V1CampaignGet(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		item, err := deps.CampaignSvc.Get(c.Context(), apiUser.ID, id)
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		return c.JSON(fiber.Map{"item": campaign(item)})
	}
}

func V1CampaignStatus(deps *ApiHandlerDeps, status sqlcdb.CampaignStatus) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		item, err := deps.CampaignSvc.SetStatus(c.Context(), apiUser.ID, id, status)
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		return c.JSON(fiber.Map{"item": campaign(item)})
	}
}

func V1CampaignEnqueue(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.JobSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "automation_unavailable"})
		}
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		var body enqueueRequest
		_ = c.BodyParser(&body)
		jobs, err := deps.JobSvc.Enqueue(c.Context(), apiUser.ID, id, body.Count)
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		out := make([]fiber.Map, 0, len(jobs))
		for _, job := range jobs {
			out = append(out, jobMap(job))
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"items": out})
	}
}

func V1CampaignJobs(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.JobSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "automation_unavailable"})
		}
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		jobs, err := deps.JobSvc.ListByCampaign(c.Context(), apiUser.ID, id, queryInt32(c, "limit", 50), queryInt32(c, "offset", 0))
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		out := make([]fiber.Map, 0, len(jobs))
		for _, job := range jobs {
			out = append(out, jobMap(job))
		}
		return c.JSON(fiber.Map{"items": out})
	}
}

func V1TargetsList(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.JobSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "automation_unavailable"})
		}
		items, err := deps.JobSvc.ListTargets(c.Context(), apiUser.ID, c.Query("pool"), queryInt32(c, "limit", 50), queryInt32(c, "offset", 0))
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		out := make([]fiber.Map, 0, len(items))
		for _, item := range items {
			out = append(out, targetMap(item))
		}
		return c.JSON(fiber.Map{"items": out})
	}
}

func V1TargetsCreate(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.JobSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "automation_unavailable"})
		}
		var body targetCreateRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		item, err := deps.JobSvc.CreateCustomTarget(c.Context(), apiUser.ID, service.CustomTargetInput(body))
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": targetMap(item)})
	}
}

func V1CampaignNext(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.JobSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "automation_unavailable"})
		}
		job, err := deps.JobSvc.ClaimNext(c.Context(), apiUser.ID)
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		return c.JSON(fiber.Map{"item": claimedJobMap(job)})
	}
}

func V1CampaignResult(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiUser, ok := middleware.ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		if deps.JobSvc == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "automation_unavailable"})
		}
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		var body jobReportRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
		}
		job, err := deps.JobSvc.Report(c.Context(), apiUser.ID, id, service.JobResultInput{
			Status:       body.Status,
			ResultURL:    body.ResultURL,
			ErrorCode:    body.ErrorCode,
			ErrorMessage: body.ErrorMessage,
			Evidence:     body.Evidence,
		})
		if err != nil {
			return writeCampaignErr(c, deps, err)
		}
		return c.JSON(fiber.Map{"item": jobMap(job)})
	}
}

func writeCampaignErr(c *fiber.Ctx, deps *ApiHandlerDeps, err error) error {
	switch {
	case errors.Is(err, service.ErrCampaignUnavailable), errors.Is(err, service.ErrJobUnavailable):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "automation_unavailable"})
	case errors.Is(err, service.ErrCampaignInvalid), errors.Is(err, service.ErrJobInvalid):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request"})
	case errors.Is(err, service.ErrCampaignNotFound), errors.Is(err, service.ErrJobNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found"})
	default:
		logAPIWarn(deps, "campaign api failed", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
	}
}

func campaignRow(row sqlcdb.ListCampaignsByUserRow) fiber.Map {
	return fiber.Map{
		"id": row.ID, "name": row.Name, "money_site_url": row.MoneySiteUrl,
		"niche_keywords": row.NicheKeywords, "anchor_texts": jsonRaw(row.AnchorTexts),
		"pool": row.Pool, "source_mode": row.SourceMode, "daily_limit": row.DailyLimit,
		"status": row.Status, "ethical_mode": row.EthicalMode, "niche_filter": row.NicheFilter,
		"credits_allocated": row.CreditsAllocated, "credits_consumed": row.CreditsConsumed,
		"started_at": formatPGTime(row.StartedAt), "completed_at": formatPGTime(row.CompletedAt),
		"created_at": row.CreatedAt.Format(time.RFC3339), "updated_at": row.UpdatedAt.Format(time.RFC3339),
		"stats": fiber.Map{"total": row.JobCount, "queued": row.QueuedCount, "dispatched": row.DispatchedCount, "in_progress": row.InProgressCount, "success": row.SuccessCount, "failed": row.FailedCount, "skipped": row.SkippedCount},
	}
}

func campaign(row sqlcdb.Campaign) fiber.Map {
	return fiber.Map{
		"id": row.ID, "name": row.Name, "money_site_url": row.MoneySiteUrl,
		"niche_keywords": row.NicheKeywords, "anchor_texts": jsonRaw(row.AnchorTexts),
		"pool": row.Pool, "source_mode": row.SourceMode, "daily_limit": row.DailyLimit,
		"status": row.Status, "ethical_mode": row.EthicalMode, "niche_filter": row.NicheFilter,
		"credits_allocated": row.CreditsAllocated, "credits_consumed": row.CreditsConsumed,
		"started_at": formatPGTime(row.StartedAt), "completed_at": formatPGTime(row.CompletedAt),
		"created_at": row.CreatedAt.Format(time.RFC3339), "updated_at": row.UpdatedAt.Format(time.RFC3339),
	}
}

func jobMap(row sqlcdb.Job) fiber.Map {
	return fiber.Map{"id": row.ID, "campaign_id": row.CampaignID, "target_id": row.TargetID, "target_url": row.TargetUrlSnapshot, "anchor_text": row.AnchorText, "anchor_type": row.AnchorType, "content_body": row.ContentBody, "content_title": row.ContentTitle, "content_meta": row.ContentMeta, "status": row.Status, "pool": row.Pool, "credits_cost": row.CreditsCost, "captcha_cost": row.CaptchaCost, "error_code": row.ErrorCode, "error_message": row.ErrorMessage, "result_url": row.ResultUrl, "dispatched_at": formatPGTime(row.DispatchedAt), "completed_at": formatPGTime(row.CompletedAt), "created_at": row.CreatedAt.Format(time.RFC3339)}
}

// claimedJobMap serializes a ClaimNextQueuedJob result. Includes money_url from
// the campaigns join so the extension worker can build the backlink without an
// extra API call. sqlc regen produces a flat row struct (not embedded Job).
func claimedJobMap(row sqlcdb.ClaimNextQueuedJobRow) fiber.Map {
	return fiber.Map{
		"id":            row.ID,
		"campaign_id":   row.CampaignID,
		"target_id":     row.TargetID,
		"target_url":    row.TargetUrlSnapshot,
		"anchor_text":   row.AnchorText,
		"anchor_type":   row.AnchorType,
		"content_body":  row.ContentBody,
		"content_title": row.ContentTitle,
		"content_meta":  row.ContentMeta,
		"status":        row.Status,
		"pool":          row.Pool,
		"credits_cost":  row.CreditsCost,
		"captcha_cost":  row.CaptchaCost,
		"error_code":    row.ErrorCode,
		"error_message": row.ErrorMessage,
		"result_url":    row.ResultUrl,
		"dispatched_at": formatPGTime(row.DispatchedAt),
		"completed_at":  formatPGTime(row.CompletedAt),
		"created_at":    row.CreatedAt.Format(time.RFC3339),
		"money_url":     row.MoneySiteUrl,
	}
}

func targetMap(row sqlcdb.Target) fiber.Map {
	return fiber.Map{"id": row.ID, "url": row.Url, "domain": row.Domain, "tld": row.Tld, "type": row.Type, "source": row.Source, "pool": row.Pool, "dr": row.Dr, "da": row.Da, "traffic_est": row.TrafficEst, "language": row.Language, "niche_tags": row.NicheTags, "platform": row.Platform, "captcha_type": row.CaptchaType, "created_at": row.CreatedAt.Format(time.RFC3339)}
}

func queryInt32(c *fiber.Ctx, key string, fallback int32) int32 {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return int32(v)
}

func jsonRaw(raw []byte) any {
	var out any
	if len(raw) == 0 || json.Unmarshal(raw, &out) != nil {
		return []any{}
	}
	return out
}

func formatPGTime(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(time.RFC3339)
	return &formatted
}

func logAPIWarn(deps *ApiHandlerDeps, msg string, err error) {
	if deps != nil && deps.Log != nil {
		deps.Log.Warn(msg, zap.Error(err))
	}
}
