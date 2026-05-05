package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// privateIPNets lists CIDR blocks that must never appear in user-supplied URLs
// to prevent SSRF when server-side verification of money_site_url is added (F13).
// Covers: loopback, RFC-1918 private ranges, link-local, and AWS metadata host.
var privateIPNets = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"::1/128",        // IPv6 loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local (AWS/GCP metadata)
		"fd00::/8",       // ULA (IPv6 private)
		"fc00::/7",       // ULA range
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, _ := net.ParseCIDR(cidr)
		if n != nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// isPrivateHost returns true when host resolves to / is a private, loopback,
// or metadata IP address. Pure hostname check (no DNS lookup) — blocks literal
// IPs; does not prevent DNS rebinding (acceptable for current threat model).
func isPrivateHost(host string) bool {
	// Strip port if present.
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	// Block "localhost" and variants before IP parsing.
	lower := strings.ToLower(strings.TrimSpace(h))
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}
	ip := net.ParseIP(lower)
	if ip == nil {
		return false // non-IP hostname: accepted (DNS rebinding risk deferred)
	}
	for _, block := range privateIPNets {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

var (
	ErrCampaignUnavailable = errors.New("campaign_unavailable")
	ErrCampaignInvalid     = errors.New("campaign_invalid")
	ErrCampaignNotFound    = errors.New("campaign_not_found")
)

type AnchorTextInput struct {
	Text   string `json:"text"`
	Weight int32  `json:"weight"`
	Type   string `json:"type"`
}

type CampaignInput struct {
	Name             string
	MoneySiteURL     string
	NicheKeywords    []string
	AnchorTexts      []AnchorTextInput
	Pool             string
	SourceMode       string
	DailyLimit       int32
	EthicalMode      bool
	NicheFilter      bool
	CreditsAllocated int32
	StartNow         bool
}

type CampaignService struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	log  *zap.Logger
}

func NewCampaignService(pool *pgxpool.Pool, q *sqlcdb.Queries, log *zap.Logger) *CampaignService {
	return &CampaignService{pool: pool, q: q, log: log}
}

func (s *CampaignService) Create(ctx context.Context, userID uuid.UUID, in CampaignInput) (sqlcdb.Campaign, error) {
	if s == nil || s.q == nil {
		return sqlcdb.Campaign{}, ErrCampaignUnavailable
	}
	if err := validateCampaignInput(in); err != nil {
		return sqlcdb.Campaign{}, err
	}
	anchors, err := json.Marshal(in.AnchorTexts)
	if err != nil {
		return sqlcdb.Campaign{}, err
	}
	status := sqlcdb.CampaignStatusDraft
	if in.StartNow {
		status = sqlcdb.CampaignStatusRunning
	}
	return s.q.CreateCampaign(ctx, sqlcdb.CreateCampaignParams{
		UserID:           userID,
		Name:             strings.TrimSpace(in.Name),
		MoneySiteUrl:     strings.TrimSpace(in.MoneySiteURL),
		NicheKeywords:    compactStrings(in.NicheKeywords, 20),
		AnchorTexts:      anchors,
		Pool:             in.Pool,
		SourceMode:       in.SourceMode,
		DailyLimit:       in.DailyLimit,
		EthicalMode:      in.EthicalMode,
		NicheFilter:      in.NicheFilter,
		CreditsAllocated: in.CreditsAllocated,
		Status:           status,
	})
}

func (s *CampaignService) List(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]sqlcdb.ListCampaignsByUserRow, error) {
	if s == nil || s.q == nil {
		return nil, ErrCampaignUnavailable
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	return s.q.ListCampaignsByUser(ctx, sqlcdb.ListCampaignsByUserParams{UserID: userID, Limit: limit, Offset: offset})
}

func (s *CampaignService) Get(ctx context.Context, userID, campaignID uuid.UUID) (sqlcdb.Campaign, error) {
	if s == nil || s.q == nil {
		return sqlcdb.Campaign{}, ErrCampaignUnavailable
	}
	c, err := s.q.GetCampaignByUser(ctx, sqlcdb.GetCampaignByUserParams{UserID: userID, ID: campaignID})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Campaign{}, ErrCampaignNotFound
	}
	return c, err
}

func (s *CampaignService) SetStatus(ctx context.Context, userID, campaignID uuid.UUID, status sqlcdb.CampaignStatus) (sqlcdb.Campaign, error) {
	if s == nil || s.q == nil {
		return sqlcdb.Campaign{}, ErrCampaignUnavailable
	}
	switch status {
	case sqlcdb.CampaignStatusRunning, sqlcdb.CampaignStatusPaused, sqlcdb.CampaignStatusCompleted, sqlcdb.CampaignStatusArchived:
	default:
		return sqlcdb.Campaign{}, ErrCampaignInvalid
	}
	c, err := s.q.UpdateCampaignStatus(ctx, sqlcdb.UpdateCampaignStatusParams{UserID: userID, ID: campaignID, Status: status})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcdb.Campaign{}, ErrCampaignNotFound
	}
	return c, err
}

func validateCampaignInput(in CampaignInput) error {
	if len(strings.TrimSpace(in.Name)) < 3 || len(strings.TrimSpace(in.Name)) > 128 {
		return ErrCampaignInvalid
	}
	u, err := url.ParseRequestURI(strings.TrimSpace(in.MoneySiteURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrCampaignInvalid
	}
	// Only http/https are allowed — reject file://, ftp://, etc.
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrCampaignInvalid
	}
	// Block private/loopback/metadata IPs to prevent SSRF when server-side
	// backlink verification is added in a future phase (F13 defensive patch).
	if isPrivateHost(u.Host) {
		return ErrCampaignInvalid
	}
	if in.Pool != "premium" && in.Pool != "standard" {
		return ErrCampaignInvalid
	}
	if in.SourceMode != "prebuilt" && in.SourceMode != "autofind" && in.SourceMode != "custom" && in.SourceMode != "mixed" {
		return ErrCampaignInvalid
	}
	if in.DailyLimit < 5 || in.DailyLimit > 50 {
		return ErrCampaignInvalid
	}
	if len(in.AnchorTexts) == 0 || len(in.AnchorTexts) > 20 {
		return ErrCampaignInvalid
	}
	for _, a := range in.AnchorTexts {
		if strings.TrimSpace(a.Text) == "" || a.Weight < 1 || a.Weight > 100 {
			return ErrCampaignInvalid
		}
		if a.Type != "branded" && a.Type != "naked" && a.Type != "generic" && a.Type != "exact" {
			return ErrCampaignInvalid
		}
	}
	return nil
}

// ResolveIDPrefix returns at most 2 campaigns matching the given ID prefix for the user.
// Bot handlers use this to resolve user-supplied short IDs (e.g. "ab12ef34") to full UUIDs.
// 0 rows → not found; 2 rows → ambiguous prefix; 1 row → resolved.
func (s *CampaignService) ResolveIDPrefix(ctx context.Context, userID uuid.UUID, prefix string) ([]sqlcdb.ResolveCampaignIDPrefixRow, error) {
	if s == nil || s.q == nil {
		return nil, ErrCampaignUnavailable
	}
	return s.q.ResolveCampaignIDPrefix(ctx, sqlcdb.ResolveCampaignIDPrefixParams{
		UserID: userID,
		Prefix: prefix,
	})
}

func compactStrings(values []string, max int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
		if len(out) >= max {
			break
		}
	}
	return out
}
