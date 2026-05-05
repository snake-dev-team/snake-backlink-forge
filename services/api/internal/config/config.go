// Package config provides typed environment variable parsing for the API service.
// Uses caarlos0/env for struct tag-based parsing with sensible defaults.
package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/caarlos0/env/v10"
)

// Config holds all environment-driven configuration for the API.
// Required fields (DATABASE_URL, REDIS_URL) will cause Load() to return an error if absent.
// Phase 2+ fields are optional — missing values skip features, not boot.
type Config struct {
	// -- Required --
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL,required"`

	// -- Runtime --
	Port     string `env:"PORT"     envDefault:"8080"`
	Env      string `env:"ENV"      envDefault:"development"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// -- Claude (Phase 2) --
	ClaudeBaseURL string `env:"CLAUDE_BASE_URL"`
	ClaudeModel   string `env:"CLAUDE_MODEL"`

	// -- Worker plane runtime (legacy execution flow) --
	WorkerSharedToken string `env:"WORKER_SHARED_TOKEN"`
	WorkerLeaseSec    int    `env:"WORKER_LEASE_SEC" envDefault:"300"`
	WorkerRetryLimit  int    `env:"WORKER_RETRY_LIMIT" envDefault:"3"`
	WorkerModel       string `env:"WORKER_MODEL"`

	// -- Telegram / Payments (Phase 2) --
	TelegramBotToken    string `env:"TELEGRAM_BOT_TOKEN"`
	TelegramBotUsername string `env:"TELEGRAM_BOT_USERNAME"`
	SepayWebhookToken   string `env:"SEPAY_WEBHOOK_TOKEN"`

	// -- SePay QR + Webhook (Phase 05-06) --
	// SepayBankCode is the SePay bank identifier used in QR URL generation (e.g. "MBBank").
	SepayBankCode string `env:"SEPAY_BANK_CODE" envDefault:"MBBank"`
	// SepayBankAccount is the destination bank account number. Never logged in full.
	SepayBankAccount string `env:"SEPAY_BANK_ACCOUNT"`

	// -- Phase 07: Installer download URL --
	// InstallerURL is the full URL to the Windows installer binary (e.g. Cloudflare R2 CDN).
	// Empty string → /download replies "installer not yet published".
	InstallerURL string `env:"INSTALLER_URL"`

	// -- External APIs (Phase 4-6) --
	SerpapiKey          string  `env:"SERPAPI_KEY"`
	MozAccessID         string  `env:"MOZ_ACCESS_ID"`
	MozSecret           string  `env:"MOZ_SECRET"`
	TwocaptchaMasterKey string  `env:"TWOCAPTCHA_MASTER_KEY"`
	CapsolverMasterKey  string  `env:"CAPSOLVER_MASTER_KEY"`
	ResendKey           string  `env:"RESEND_KEY"`
	JWTSecret           string  `env:"JWT_SECRET"`
	AdminTelegramIDs    []int64 `env:"ADMIN_TELEGRAM_IDS" envSeparator:","`

	// -- Web SaaS API (Phase 3) --
	CORSOrigins       []string `env:"CORS_ORIGINS"        envSeparator:","`
	TrustedProxyCIDRs []string `env:"TRUSTED_PROXY_CIDRS" envSeparator:"," envDefault:"fdaa::/16,100.64.0.0/10,127.0.0.1/32"`
	WPEncKey          string   `env:"WP_ENC_KEY"`
}

// Load parses environment variables into a Config struct.
// Returns an error if required variables are missing or malformed.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: parse env: %w", err)
	}

	// Normalize env value for predictable comparisons downstream.
	cfg.Env = strings.ToLower(strings.TrimSpace(cfg.Env))
	cfg.ClaudeBaseURL = strings.TrimSpace(cfg.ClaudeBaseURL)
	cfg.ClaudeModel = strings.TrimSpace(cfg.ClaudeModel)
	cfg.WorkerSharedToken = strings.TrimSpace(cfg.WorkerSharedToken)
	cfg.WorkerModel = strings.TrimSpace(cfg.WorkerModel)
	cfg.SepayWebhookToken = strings.TrimSpace(cfg.SepayWebhookToken)
	cfg.SepayBankAccount = strings.TrimSpace(cfg.SepayBankAccount)
	cfg.CORSOrigins = normalizeStringSlice(cfg.CORSOrigins)
	cfg.TrustedProxyCIDRs = normalizeStringSlice(cfg.TrustedProxyCIDRs)

	if err := validateCORSOrigins(cfg.CORSOrigins); err != nil {
		return nil, fmt.Errorf("config: CORS_ORIGINS: %w", err)
	}
	if err := validateSePayWebhookConfig(cfg); err != nil {
		return nil, fmt.Errorf("config: SePay webhook: %w", err)
	}
	if err := validateWorkerRuntime(cfg); err != nil {
		return nil, fmt.Errorf("config: worker runtime: %w", err)
	}

	// [L3] Validate + deduplicate admin telegram IDs after env.Parse populates the slice.
	validated, err := validateAdminTelegramIDs(cfg.AdminTelegramIDs)
	if err != nil {
		return nil, fmt.Errorf("config: ADMIN_TELEGRAM_IDS: %w", err)
	}
	cfg.AdminTelegramIDs = validated

	return cfg, nil
}

// validateAdminTelegramIDs deduplicates and validates the parsed admin ID slice.
// Rules:
//   - Non-positive values → error (fail boot)
//   - Duplicate values → warn to stderr (keep first occurrence)
//   - Result is sorted ascending for deterministic iteration
func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func validateCORSOrigins(origins []string) error {
	for _, origin := range origins {
		if strings.ContainsAny(origin, "*()[]{}") {
			return fmt.Errorf("wildcards or regex-like chars are banned with credentials: %q", origin)
		}
	}
	return nil
}

func validateSePayWebhookConfig(cfg *Config) error {
	if len(cfg.SepayWebhookToken) < 32 {
		return fmt.Errorf("SEPAY_WEBHOOK_TOKEN must be at least 32 characters")
	}
	if cfg.SepayBankAccount == "" {
		return fmt.Errorf("SEPAY_BANK_ACCOUNT is required")
	}
	return nil
}

func validateWorkerRuntime(cfg *Config) error {
	if cfg.WorkerLeaseSec <= 0 {
		return fmt.Errorf("WORKER_LEASE_SEC must be positive")
	}
	if cfg.WorkerRetryLimit < 0 {
		return fmt.Errorf("WORKER_RETRY_LIMIT must be zero or positive")
	}
	if cfg.WorkerModel == "" {
		cfg.WorkerModel = cfg.ClaudeModel
	}
	return nil
}

func validateAdminTelegramIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("non-positive value: %d", id)
		}
		if _, dup := seen[id]; dup {
			// Log warn to stderr — logger not yet constructed at config load time.
			fmt.Fprintf(os.Stderr, "config: duplicate ADMIN_TELEGRAM_ID %d (kept first occurrence)\n", id)
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// IsProduction returns true when running in production mode.
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
