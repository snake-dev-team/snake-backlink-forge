package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// evidenceMaxBytes caps the evidence payload accepted by validateBacklinkEvidence.
// 16 KB is sufficient for any real DOM snippet; 200 KB was an unnecessarily wide
// window that invited memory pressure under concurrent reporting (F9).
const evidenceMaxBytes = 16_384

var (
	ErrJobUnavailable = errors.New("job_unavailable")
	ErrJobInvalid     = errors.New("job_invalid")
	ErrJobNotFound    = errors.New("job_not_found")
)

type CustomTargetInput struct {
	URL       string
	Type      string
	Pool      string
	Language  string
	NicheTags []string
	Platform  string
}

type JobResultInput struct {
	ResultURL    string
	ErrorCode    string
	ErrorMessage string
	Evidence     string
	Status       string
}

type JobService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	log  *zap.Logger
}

func NewJobService(pool *pgxpool.Pool, q *sqlcdb.Queries, log *zap.Logger) *JobService {
	return &JobService{pool: pool, q: q, log: log}
}

func (s *JobService) CreateCustomTarget(ctx context.Context, userID uuid.UUID, in CustomTargetInput) (sqlcdb.Target, error) {
	if s == nil || s.q == nil {
		return sqlcdb.Target{}, ErrJobUnavailable
	}
	u, err := url.ParseRequestURI(strings.TrimSpace(in.URL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return sqlcdb.Target{}, ErrJobInvalid
	}
	typeValue, ok := parseTargetType(in.Type)
	if !ok {
		return sqlcdb.Target{}, ErrJobInvalid
	}
	pool := in.Pool
	if pool != "premium" && pool != "standard" {
		pool = "standard"
	}
	owner := userID
	var lang *string
	if strings.TrimSpace(in.Language) != "" {
		v := strings.TrimSpace(in.Language)
		lang = &v
	}
	var platform *string
	if strings.TrimSpace(in.Platform) != "" {
		v := strings.TrimSpace(in.Platform)
		platform = &v
	}
	return s.q.CreateCustomTarget(ctx, sqlcdb.CreateCustomTargetParams{
		Url:           strings.TrimSpace(in.URL),
		Domain:        strings.ToLower(u.Hostname()),
		Tld:           tldFromHost(u.Hostname()),
		Type:          typeValue,
		OwnerUserID:   &owner,
		Pool:          pool,
		Language:      lang,
		NicheTags:     compactStrings(in.NicheTags, 20),
		Platform:      platform,
		FormSelectors: json.RawMessage(`{}`),
	})
}

func (s *JobService) ListTargets(ctx context.Context, userID uuid.UUID, pool string, limit, offset int32) ([]sqlcdb.Target, error) {
	if s == nil || s.q == nil {
		return nil, ErrJobUnavailable
	}
	if pool != "premium" && pool != "standard" {
		pool = ""
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	owner := userID
	// Column3 is the target-type filter; frontend never sends it so we pass ""
	// which the SQL short-circuits via ($3::text = '' OR ...). Explicit zero avoids
	// implicit struct-zeroing masking future query changes (F6).
	return s.q.ListTargetsForUser(ctx, sqlcdb.ListTargetsForUserParams{OwnerUserID: &owner, Column2: pool, Column3: "", Limit: limit, Offset: offset})
}

func (s *JobService) Enqueue(ctx context.Context, userID, campaignID uuid.UUID, count int32) ([]sqlcdb.Job, error) {
	if s == nil || s.pool == nil || s.q == nil {
		return nil, ErrJobUnavailable
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("job_service.Enqueue: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	campaign, err := qtx.GetCampaignByUser(ctx, sqlcdb.GetCampaignByUserParams{UserID: userID, ID: campaignID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCampaignNotFound
	}
	if err != nil {
		return nil, err
	}
	if count <= 0 || count > campaign.DailyLimit {
		count = campaign.DailyLimit
	}
	anchors, err := decodeAnchors(campaign.AnchorTexts)
	if err != nil || len(anchors) == 0 {
		return nil, ErrJobInvalid
	}
	queuedCost, err := qtx.CountCampaignQueuedWork(ctx, sqlcdb.CountCampaignQueuedWorkParams{UserID: userID, CampaignID: campaign.ID})
	if err != nil {
		return nil, err
	}
	completedCost, err := qtx.CountCampaignCompletedWork(ctx, sqlcdb.CountCampaignCompletedWorkParams{UserID: userID, CampaignID: campaign.ID})
	if err != nil {
		return nil, err
	}
	remainingBudget := campaign.CreditsAllocated - campaign.CreditsConsumed - queuedCost - completedCost
	if remainingBudget <= 0 {
		return nil, ErrInsufficientCredits
	}
	if count > remainingBudget {
		count = remainingBudget
	}

	// Decide path: if campaign has linked wp_sites, use them. Otherwise fall back to
	// global targets pool (legacy campaigns created before campaign_target_sites table).
	siteCount, err := qtx.CountCampaignTargetSites(ctx, campaign.ID)
	if err != nil {
		return nil, fmt.Errorf("job_service.Enqueue: count sites: %w", err)
	}

	var jobs []sqlcdb.Job

	if siteCount > 0 {
		// ── New path: jobs from user's own wp_sites via campaign_target_sites ──────
		// One job per linked wp_site (deduplication: skip if job already exists for that base_url).
		// No UpsertDomainCooldown: user owns the site so per-domain cooldown is irrelevant.
		sites, err := qtx.PickWpSitesForCampaign(ctx, sqlcdb.PickWpSitesForCampaignParams{
			CampaignID: campaign.ID,
			UserID:     userID,
			LimitCount: count,
		})
		if err != nil {
			return nil, fmt.Errorf("job_service.Enqueue: pick wp sites: %w", err)
		}
		if len(sites) == 0 {
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("job_service.Enqueue: commit empty sites: %w", err)
			}
			return []sqlcdb.Job{}, nil
		}

		jobs = make([]sqlcdb.Job, 0, len(sites))
		for i, site := range sites {
			anchor := anchors[i%len(anchors)]
			job, err := qtx.CreateJobForSite(ctx, sqlcdb.CreateJobForSiteParams{
				UserID:            userID,
				CampaignID:        campaign.ID,
				TargetUrlSnapshot: site.BaseUrl,
				AnchorText:        anchor.Text,
				AnchorType:        anchor.Type,
				Pool:              campaign.Pool,
				CreditsCost:       1,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				// ON CONFLICT DO NOTHING — site already has a job for this campaign; skip.
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("job_service.Enqueue: create job for site %s: %w", site.BaseUrl, err)
			}
			jobs = append(jobs, job)
		}
	} else {
		// ── Legacy path: pick from global targets pool ────────────────────────────
		targets, err := qtx.PickTargetsForCampaign(ctx, sqlcdb.PickTargetsForCampaignParams{
			OwnerUserID: &userID,
			Pool:        campaign.Pool,
			CampaignID:  campaign.ID,
			Column4:     24,
			Limit:       count,
		})
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("job_service.Enqueue: commit empty: %w", err)
			}
			return []sqlcdb.Job{}, nil
		}

		// F1 fix: INSERT jobs FIRST, then debit only the actual inserted count.
		// The original code debited len(targets) before the insert loop, which caused
		// silent overcharges when ON CONFLICT DO NOTHING skipped duplicates.
		// Now: jobs slice accumulates successes → actual cost = len(jobs) → charge exactly that.
		jobs = make([]sqlcdb.Job, 0, len(targets))
		for i, target := range targets {
			anchor := anchors[i%len(anchors)]
			tID := target.ID // copy for pointer
			job, err := qtx.CreateJob(ctx, sqlcdb.CreateJobParams{
				UserID:            userID,
				CampaignID:        campaign.ID,
				TargetID:          &tID,
				TargetUrlSnapshot: target.Url,
				AnchorText:        anchor.Text,
				AnchorType:        anchor.Type,
				Pool:              campaign.Pool,
				CreditsCost:       1,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				// ON CONFLICT DO NOTHING — duplicate target for this campaign; skip.
				continue
			}
			if err != nil {
				return nil, err
			}
			if err := qtx.UpsertDomainCooldown(ctx, sqlcdb.UpsertDomainCooldownParams{UserID: userID, Domain: target.Domain}); err != nil {
				return nil, err
			}
			jobs = append(jobs, job)
		}
	}

	// If all targets were duplicates, roll back without charging anything.
	if len(jobs) == 0 {
		// defer tx.Rollback cleans up; no credits have been debited yet.
		return nil, ErrJobInvalid
	}

	// Debit credits equal to the ACTUAL number of jobs inserted (not the requested count).
	actualCost := int32(len(jobs))
	var balanceAfter int
	if err := tx.QueryRow(ctx, `SELECT consume_credits($1,$2,$3,$4,$5,$6)`, userID, campaign.Pool, actualCost, "consume_backlink", "campaign", campaign.ID).Scan(&balanceAfter); err != nil {
		if isInsufficientCredits(err) {
			return nil, ErrInsufficientCredits
		}
		return nil, fmt.Errorf("job_service.Enqueue: consume credits: %w", err)
	}

	// F7 fix: bump campaigns.credits_consumed at enqueue by the actual inserted count.
	// The Report(success) path ALSO calls BumpCampaignCreditsConsumed, which tracks
	// "confirmed delivered" separately. Both fields serve different semantics:
	// - credits_consumed here = credits committed (wallet debited, job queued)
	// - credits_consumed on success = backlinks actually placed (for delivery metrics)
	// UI displays credits_consumed so users see charges reflected immediately at enqueue.
	if err := qtx.BumpCampaignCreditsConsumed(ctx, sqlcdb.BumpCampaignCreditsConsumedParams{
		UserID:          userID,
		ID:              campaign.ID,
		CreditsConsumed: actualCost,
	}); err != nil {
		return nil, fmt.Errorf("job_service.Enqueue: bump credits_consumed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("job_service.Enqueue: commit: %w", err)
	}
	return jobs, nil
}

func (s *JobService) ListByCampaign(ctx context.Context, userID, campaignID uuid.UUID, limit, offset int32) ([]sqlcdb.Job, error) {
	if s == nil || s.q == nil {
		return nil, ErrJobUnavailable
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.q.ListJobsByCampaign(ctx, sqlcdb.ListJobsByCampaignParams{UserID: userID, CampaignID: campaignID, Limit: limit, Offset: offset})
}

func (s *JobService) ClaimNext(ctx context.Context, userID uuid.UUID) (sqlcdb.ClaimNextQueuedJobRow, error) {
	if s == nil || s.q == nil {
		return sqlcdb.ClaimNextQueuedJobRow{}, ErrJobUnavailable
	}
	job, err := s.q.ClaimNextQueuedJob(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.ClaimNextQueuedJobRow{}, ErrJobNotFound
	}
	return job, err
}

func (s *JobService) Report(ctx context.Context, userID, jobID uuid.UUID, in JobResultInput) (sqlcdb.Job, error) {
	if s == nil || s.pool == nil || s.q == nil {
		return sqlcdb.Job{}, ErrJobUnavailable
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return sqlcdb.Job{}, fmt.Errorf("job_service.Report: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	claimed, err := qtx.GetJobForReport(ctx, sqlcdb.GetJobForReportParams{UserID: userID, ID: jobID})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Job{}, ErrJobNotFound
	}
	if err != nil {
		return sqlcdb.Job{}, err
	}
	status, ok := normalizeReportStatus(in.Status)
	if !ok {
		return sqlcdb.Job{}, ErrJobInvalid
	}
	if status == "success" {
		result, err := validateResultURL(in.ResultURL, claimed.TargetUrlSnapshot)
		if err != nil {
			return sqlcdb.Job{}, err
		}
		campaign, err := qtx.GetCampaignByUser(ctx, sqlcdb.GetCampaignByUserParams{UserID: userID, ID: claimed.CampaignID})
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcdb.Job{}, ErrCampaignNotFound
		}
		if err != nil {
			return sqlcdb.Job{}, err
		}
		if err := validateBacklinkEvidence(in.Evidence, campaign.MoneySiteUrl, claimed.AnchorText); err != nil {
			return sqlcdb.Job{}, err
		}
		job, err := qtx.CompleteJob(ctx, sqlcdb.CompleteJobParams{UserID: userID, ID: jobID, ResultUrl: result})
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcdb.Job{}, ErrJobNotFound
		}
		if err != nil {
			return sqlcdb.Job{}, err
		}
		if err := qtx.BumpCampaignCreditsConsumed(ctx, sqlcdb.BumpCampaignCreditsConsumedParams{UserID: userID, ID: job.CampaignID, CreditsConsumed: job.CreditsCost}); err != nil {
			return sqlcdb.Job{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return sqlcdb.Job{}, fmt.Errorf("job_service.Report: commit success: %w", err)
		}
		return job, nil
	}
	code := strings.TrimSpace(in.ErrorCode)
	if code == "" {
		if status == "skipped" {
			code = "job_skipped"
		} else {
			code = "job_failed"
		}
	}
	msg := strings.TrimSpace(in.ErrorMessage)
	var job sqlcdb.Job
	if status == "skipped" {
		job, err = qtx.SkipJob(ctx, sqlcdb.SkipJobParams{UserID: userID, ID: jobID, ErrorCode: &code, ErrorMessage: &msg})
	} else {
		job, err = qtx.FailJob(ctx, sqlcdb.FailJobParams{UserID: userID, ID: jobID, ErrorCode: &code, ErrorMessage: &msg})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Job{}, ErrJobNotFound
	}
	if err != nil {
		return sqlcdb.Job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlcdb.Job{}, fmt.Errorf("job_service.Report: commit fail: %w", err)
	}
	return job, nil
}

func decodeAnchors(raw []byte) ([]AnchorTextInput, error) {
	var anchors []AnchorTextInput
	if err := json.Unmarshal(raw, &anchors); err != nil {
		return nil, err
	}
	return anchors, nil
}

func parseTargetType(value string) (sqlcdb.TargetType, bool) {
	switch value {
	case string(sqlcdb.TargetTypeBlogComment):
		return sqlcdb.TargetTypeBlogComment, true
	case string(sqlcdb.TargetTypeForumProfile):
		return sqlcdb.TargetTypeForumProfile, true
	case string(sqlcdb.TargetTypeWeb2Post):
		return sqlcdb.TargetTypeWeb2Post, true
	case string(sqlcdb.TargetTypeDirectoryListing):
		return sqlcdb.TargetTypeDirectoryListing, true
	default:
		return "", false
	}
}

func tldFromHost(host string) *string {
	parts := strings.Split(strings.ToLower(host), ".")
	if len(parts) < 2 {
		return nil
	}
	v := parts[len(parts)-1]
	return &v
}

func validateResultURL(resultURL, targetURL string) (*string, error) {
	result := strings.TrimSpace(resultURL)
	if result == "" {
		return nil, ErrJobInvalid
	}
	parsedResult, err := url.ParseRequestURI(result)
	if err != nil || parsedResult.Scheme == "" || parsedResult.Host == "" {
		return nil, ErrJobInvalid
	}
	if parsedResult.Scheme != "http" && parsedResult.Scheme != "https" {
		return nil, ErrJobInvalid
	}
	parsedTarget, err := url.ParseRequestURI(strings.TrimSpace(targetURL))
	if err != nil || !sameHost(parsedResult.Hostname(), parsedTarget.Hostname()) {
		return nil, ErrJobInvalid
	}
	return &result, nil
}

func validateBacklinkEvidence(evidence, moneySiteURL, anchorText string) error {
	normalizedEvidence := strings.ToLower(strings.TrimSpace(evidence))
	// evidenceMaxBytes (16 KB) is sufficient for any real DOM snippet (F9).
	if normalizedEvidence == "" || len(normalizedEvidence) > evidenceMaxBytes {
		return ErrJobInvalid
	}
	moneyURL := strings.ToLower(strings.TrimSpace(moneySiteURL))
	anchor := strings.ToLower(strings.TrimSpace(anchorText))
	if moneyURL == "" || anchor == "" {
		return ErrJobInvalid
	}
	if !strings.Contains(normalizedEvidence, moneyURL) || !strings.Contains(normalizedEvidence, anchor) {
		return ErrJobInvalid
	}
	return nil
}

func normalizeReportStatus(value string) (string, bool) {
	status := strings.TrimSpace(strings.ToLower(value))
	switch status {
	case "success", "failed", "skipped":
		return status, true
	default:
		return "", false
	}
}

func sameHost(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func isInsufficientCredits(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, ErrInsufficientCredits) || (errors.As(err, &pgErr) && pgErr.Code == "P0001" && pgErr.Message == "INSUFFICIENT_CREDITS")
}

// GetCampaignJobStats returns aggregated job status counts for a campaign.
// Thin wrapper around the CampaignJobStats sqlc query for use by bot handlers.
func (s *JobService) GetCampaignJobStats(ctx context.Context, userID, campaignID uuid.UUID) (sqlcdb.CampaignJobStatsRow, error) {
	if s == nil || s.q == nil {
		return sqlcdb.CampaignJobStatsRow{}, ErrJobUnavailable
	}
	return s.q.CampaignJobStats(ctx, sqlcdb.CampaignJobStatsParams{
		UserID:     userID,
		CampaignID: campaignID,
	})
}
