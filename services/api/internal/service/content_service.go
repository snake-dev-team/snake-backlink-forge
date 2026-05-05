package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/ai"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// Cost constants for Claude Sonnet (claude-sonnet-4-6).
// Prices per million tokens; used to estimate and cap per-article spend.
const (
	claudeCostPerMInputUSD  = 3.0  // $3.00 / 1M input tokens
	claudeCostPerMOutputUSD = 15.0 // $15.00 / 1M output tokens

	// maxCostPerArticleUSD is the hard cap; generation is aborted if exceeded.
	// Sized for Claude Opus 4.7 worst case (~4K output tokens × $15/M = $0.06)
	// plus headroom for proxy markup. Sonnet typical run: $0.025-0.035.
	maxCostPerArticleUSD = 0.15

	// wordCountMin / wordCountMax define acceptable article lengths.
	wordCountMin = 800
	wordCountMax = 1500

	// maxQualityRetries is the max number of regeneration attempts on guard failure.
	maxQualityRetries = 1
)

// aiPhraseRe matches common AI-generated filler phrases (case-insensitive, RE2).
var aiPhraseRe = regexp.MustCompile(`(?i)\b(as an? ai|i cannot|i can't|i'm unable|it's important to note|however, it's worth)\b`)

var (
	ErrContentUnavailable = errors.New("content_unavailable")
	ErrContentInvalid     = errors.New("content_invalid")
)

// ContentService orchestrates AI article generation for campaign jobs.
// It reads queued jobs, generates articles via the AI router, applies quality
// guards, and writes content_body/title/meta + transitions status to content_ready.
type ContentService struct {
	pool      *pgxpool.Pool
	q         *sqlcdb.Queries
	generator ai.Generator
	log       *zap.Logger
}

// NewContentService constructs a ContentService.
// generator may be a Router (primary+fallback) or a single client.
// A nil log is replaced with a no-op logger so handlers and tests can pass nil safely.
func NewContentService(pool *pgxpool.Pool, q *sqlcdb.Queries, generator ai.Generator, log *zap.Logger) *ContentService {
	if log == nil {
		log = zap.NewNop()
	}
	return &ContentService{pool: pool, q: q, generator: generator, log: log}
}

// GenerateResult summarises the outcome of a GenerateForCampaign call.
type GenerateResult struct {
	// JobsProcessed is the count of jobs that received content_ready status.
	JobsProcessed int
	// EstimatedCostUSD is the total inferred spend across all processed jobs.
	EstimatedCostUSD float64
}

// GenerateForCampaign loops all queued jobs of a campaign that have no content,
// generates AI articles for each, applies quality guards, and persists results.
// It is safe to call concurrently: each job UPDATE is gated by status='queued'.
func (s *ContentService) GenerateForCampaign(
	ctx context.Context,
	userID, campaignID uuid.UUID,
	tonePreference string,
) (GenerateResult, error) {
	if s == nil || s.q == nil || s.generator == nil {
		return GenerateResult{}, ErrContentUnavailable
	}

	// Fetch campaign to get money_site_url + anchor_texts.
	campaign, err := s.q.GetCampaignByUser(ctx, sqlcdb.GetCampaignByUserParams{
		UserID: userID,
		ID:     campaignID,
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("content_service: get campaign: %w", err)
	}

	anchors, err := decodeAnchors(campaign.AnchorTexts)
	if err != nil || len(anchors) == 0 {
		return GenerateResult{}, ErrContentInvalid
	}

	// Fetch all queued jobs that need content.
	jobs, err := s.q.GetCampaignQueuedJobsForContent(ctx, sqlcdb.GetCampaignQueuedJobsForContentParams{
		CampaignID: campaignID,
		UserID:     userID,
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("content_service: fetch jobs: %w", err)
	}

	// Derive topic from campaign niche keywords (first keyword) or campaign name.
	topic := deriveTopicFromCampaign(campaign)

	var result GenerateResult

	for i, job := range jobs {
		// Context cancellation check between jobs.
		if ctx.Err() != nil {
			s.log.Warn("content_service: context cancelled mid-batch",
				zap.Int("processed", result.JobsProcessed),
				zap.Int("remaining", len(jobs)-i),
			)
			break
		}

		// Rotate anchors using same pattern as Enqueue (anchors[i%len(anchors)]).
		anchor := anchors[i%len(anchors)]

		req := ai.ContentRequest{
			Topic:          topic,
			AnchorText:     anchor.Text,
			AnchorType:     anchor.Type,
			MoneyURL:       campaign.MoneySiteUrl,
			TonePreference: tonePreference,
		}

		resp, costUSD, err := s.generateWithQualityGuard(ctx, req, anchor.Text, campaign.MoneySiteUrl)
		if err != nil {
			s.log.Warn("content_service: generation failed for job",
				zap.String("job_id", job.ID.String()),
				zap.Error(err),
			)
			// Non-fatal: continue processing remaining jobs.
			continue
		}

		// Persist content + transition to content_ready.
		if updateErr := s.q.UpdateJobContent(ctx, sqlcdb.UpdateJobContentParams{
			ID:           job.ID,
			UserID:       userID,
			ContentBody:  resp.BodyHTML,
			ContentTitle: resp.Title,
			ContentMeta:  resp.MetaDescription,
		}); updateErr != nil {
			s.log.Warn("content_service: update job content failed",
				zap.String("job_id", job.ID.String()),
				zap.Error(updateErr),
			)
			continue
		}

		result.JobsProcessed++
		result.EstimatedCostUSD += costUSD
	}

	return result, nil
}

// generateWithQualityGuard calls the AI generator and validates the output.
// Retries once on quality guard failure (maxQualityRetries=1).
// Returns the response, cost in USD, and any error.
func (s *ContentService) generateWithQualityGuard(
	ctx context.Context,
	req ai.ContentRequest,
	anchorText, moneyURL string,
) (ai.ContentResponse, float64, error) {
	var lastErr error

	for attempt := 0; attempt <= maxQualityRetries; attempt++ {
		resp, err := s.generator.Generate(ctx, req)
		if err != nil {
			return ai.ContentResponse{}, 0, err
		}

		costUSD := calcCostUSD(resp.InputTokens, resp.OutputTokens)
		if costUSD > maxCostPerArticleUSD {
			return ai.ContentResponse{}, 0, ai.ErrAIQuotaExceeded
		}

		if guardErr := validateContent(resp.BodyHTML, anchorText, moneyURL); guardErr != nil {
			lastErr = guardErr
			s.log.Warn("content_service: quality guard failed, retrying",
				zap.Int("attempt", attempt+1),
				zap.Error(guardErr),
			)
			continue
		}

		return resp, costUSD, nil
	}

	return ai.ContentResponse{}, 0, fmt.Errorf("%w: %v", ai.ErrQualityRejected, lastErr)
}

// validateContent checks generated body HTML against quality rules:
//  1. Word count 800-1500
//  2. Anchor text present (case-insensitive)
//  3. Money URL present
//  4. No AI filler phrases
func validateContent(bodyHTML, anchorText, moneyURL string) error {
	wc := ai.WordCount(bodyHTML)
	if wc < wordCountMin || wc > wordCountMax {
		return fmt.Errorf("word count %d not in [%d,%d]", wc, wordCountMin, wordCountMax)
	}

	lower := strings.ToLower(bodyHTML)
	if !strings.Contains(lower, strings.ToLower(anchorText)) {
		return fmt.Errorf("anchor text %q not found in body", anchorText)
	}
	if !strings.Contains(lower, strings.ToLower(moneyURL)) {
		return fmt.Errorf("money URL %q not found in body", moneyURL)
	}

	if aiPhraseRe.MatchString(bodyHTML) {
		return fmt.Errorf("AI filler phrase detected in body")
	}

	return nil
}

// calcCostUSD returns the estimated cost in USD for a single generation call.
// Uses Claude Sonnet pricing: $3/M input, $15/M output.
func calcCostUSD(inputTokens, outputTokens int) float64 {
	return (float64(inputTokens)/1_000_000)*claudeCostPerMInputUSD +
		(float64(outputTokens)/1_000_000)*claudeCostPerMOutputUSD
}

// CountQueuedJobsNeedingContent returns the number of queued jobs for the
// campaign that still have no content. Used by the handler to populate the
// 202 response body before spawning the background goroutine.
func (s *ContentService) CountQueuedJobsNeedingContent(ctx context.Context, userID, campaignID uuid.UUID) (int, error) {
	if s == nil || s.q == nil {
		return 0, ErrContentUnavailable
	}
	jobs, err := s.q.GetCampaignQueuedJobsForContent(ctx, sqlcdb.GetCampaignQueuedJobsForContentParams{
		CampaignID: campaignID,
		UserID:     userID,
	})
	if err != nil {
		return 0, fmt.Errorf("content_service: count queued jobs: %w", err)
	}
	return len(jobs), nil
}

// deriveTopicFromCampaign returns a human-readable topic string for the prompt.
// Uses first niche keyword if available, falls back to campaign name.
func deriveTopicFromCampaign(c sqlcdb.Campaign) string {
	if len(c.NicheKeywords) > 0 && strings.TrimSpace(c.NicheKeywords[0]) != "" {
		return strings.TrimSpace(c.NicheKeywords[0])
	}
	return strings.TrimSpace(c.Name)
}
