package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/util"
	"go.uber.org/zap"
)

var (
	ErrWpInvalidURL             = errors.New("wp_invalid_url")
	ErrWpInvalidCredentials     = errors.New("wp_invalid_credentials")
	ErrWpInsufficientCapability = errors.New("wp_insufficient_capability")
	ErrWpRestAPINotFound        = errors.New("wp_rest_api_not_found")
	ErrWpRateLimited            = errors.New("wp_rate_limited")
	ErrWpServerError502         = errors.New("wp_server_error_502")
	ErrWpServerError503         = errors.New("wp_server_error_503")
	ErrWpServerError504         = errors.New("wp_server_error_504")
	ErrWpPrivateAddress         = errors.New("wp_private_address")
	// ErrWPSiteNotFound is returned by GetByDomainPlain when no connected wp_site matches.
	ErrWPSiteNotFound = errors.New("wp_site_not_found")
)

type WpSiteService struct {
	pool      *pgxpool.Pool
	q         *sqlcdb.Queries
	masterKey string
	client    *http.Client
	log       *zap.Logger
}

type WpSiteInput struct {
	BaseURL     string
	AppUsername string
	AppPassword string
	Label       string
}

func NewWpSiteService(pool *pgxpool.Pool, q *sqlcdb.Queries, masterKey string, log *zap.Logger) *WpSiteService {
	return &WpSiteService{
		pool:      pool,
		q:         q,
		masterKey: masterKey,
		client:    newWPHTTPClient(),
		log:       log,
	}
}

func (s *WpSiteService) Create(ctx context.Context, userID uuid.UUID, in WpSiteInput) (sqlcdb.WpSite, error) {
	baseURL, err := NormalizeWPBaseURL(in.BaseURL)
	if err != nil {
		return sqlcdb.WpSite{}, err
	}
	if err := s.Validate(ctx, baseURL, in.AppUsername, in.AppPassword); err != nil {
		return sqlcdb.WpSite{}, err
	}
	enc, err := util.EncryptAESGCM(s.masterKey, []byte(strings.TrimSpace(in.AppPassword)))
	if err != nil {
		return sqlcdb.WpSite{}, fmt.Errorf("wp_site.Create: encrypt: %w", err)
	}
	return s.q.InsertWpSite(ctx, sqlcdb.InsertWpSiteParams{
		UserID:          userID,
		BaseUrl:         baseURL,
		AppUsername:     strings.TrimSpace(in.AppUsername),
		AppPasswordEnc:  enc,
		Label:           strings.TrimSpace(in.Label),
		Status:          sqlcdb.WpSiteStatusConnected,
		LastValidatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
}

func (s *WpSiteService) List(ctx context.Context, userID uuid.UUID) ([]sqlcdb.ListWpSitesByUserRow, error) {
	return s.q.ListWpSitesByUser(ctx, userID)
}

func (s *WpSiteService) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	return s.q.SoftDeleteWpSite(ctx, sqlcdb.SoftDeleteWpSiteParams{ID: id, UserID: userID})
}

func (s *WpSiteService) Revalidate(ctx context.Context, userID uuid.UUID, id uuid.UUID) (sqlcdb.WpSiteStatus, error) {
	site, err := s.q.GetWpSiteByID(ctx, sqlcdb.GetWpSiteByIDParams{ID: id, UserID: userID})
	if err != nil {
		return "", err
	}
	password, err := util.DecryptAESGCM(s.masterKey, site.AppPasswordEnc)
	if err != nil {
		return "", fmt.Errorf("wp_site.Revalidate: decrypt: %w", err)
	}
	status := sqlcdb.WpSiteStatusConnected
	var lastErr *string
	if err := s.Validate(ctx, site.BaseUrl, site.AppUsername, string(password)); err != nil {
		status = sqlcdb.WpSiteStatusError
		errText := WpErrorCode(err)
		lastErr = &errText
	}
	if err := s.q.UpdateWpSiteStatus(ctx, sqlcdb.UpdateWpSiteStatusParams{Status: status, LastError: lastErr, ID: id, UserID: userID}); err != nil {
		return "", err
	}
	return status, nil
}

func NormalizeWPBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", ErrWpInvalidURL
	}
	if strings.ToLower(u.Scheme) != "https" {
		return "", ErrWpInvalidURL
	}
	if hostIP := net.ParseIP(u.Hostname()); hostIP != nil && !isPublicIP(hostIP) {
		return "", ErrWpPrivateAddress
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.EscapedPath(), "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func WpErrorCode(err error) string {
	for _, target := range []error{ErrWpInvalidURL, ErrWpInvalidCredentials, ErrWpInsufficientCapability, ErrWpRestAPINotFound, ErrWpRateLimited, ErrWpServerError502, ErrWpServerError503, ErrWpServerError504, ErrWpPrivateAddress} {
		if errors.Is(err, target) {
			return target.Error()
		}
	}
	return "wp_network"
}

func wpStatusError(status int) error {
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return ErrWpInvalidCredentials
	case http.StatusForbidden:
		return ErrWpInsufficientCapability
	case http.StatusNotFound:
		return ErrWpRestAPINotFound
	case http.StatusTooManyRequests:
		return ErrWpRateLimited
	case http.StatusBadGateway:
		return ErrWpServerError502
	case http.StatusServiceUnavailable:
		return ErrWpServerError503
	case http.StatusGatewayTimeout:
		return ErrWpServerError504
	default:
		if status >= 500 {
			return fmt.Errorf("wp_server_error_%d", status)
		}
		return fmt.Errorf("wp_unexpected_status_%d", status)
	}
}

// WPSiteCredsView holds decrypted credentials returned to the extension.
// Never logged — caller must ensure response body is not captured in access logs.
type WPSiteCredsView struct {
	BaseURL          string `json:"base_url"`
	AppUsername      string `json:"app_username"`
	AppPasswordPlain string `json:"app_password_plain"`
}

// GetByDomainPlain looks up the first connected wp_site for the user matching domain,
// decrypts the Application Password, and returns plaintext credentials.
// Returns ErrWPSiteNotFound if no matching connected site exists.
// Used exclusively by the extension /wp-sites/by-domain/:domain signed endpoint.
func (s *WpSiteService) GetByDomainPlain(ctx context.Context, userID uuid.UUID, domain string) (WPSiteCredsView, error) {
	row, err := s.q.GetWPSiteByUserDomain(ctx, sqlcdb.GetWPSiteByUserDomainParams{
		UserID: userID,
		Domain: domain,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return WPSiteCredsView{}, ErrWPSiteNotFound
	}
	if err != nil {
		return WPSiteCredsView{}, fmt.Errorf("wp_site.GetByDomainPlain: query: %w", err)
	}
	plain, err := util.DecryptAESGCM(s.masterKey, row.AppPasswordEnc)
	if err != nil {
		return WPSiteCredsView{}, fmt.Errorf("wp_site.GetByDomainPlain: decrypt: %w", err)
	}
	return WPSiteCredsView{
		BaseURL:          row.BaseUrl,
		AppUsername:      row.AppUsername,
		AppPasswordPlain: string(plain),
	}, nil
}
