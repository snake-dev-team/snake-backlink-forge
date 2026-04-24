// Package config provides typed environment variable parsing for the API service.
// Uses caarlos0/env for struct tag-based parsing with sensible defaults.
package config

import (
	"fmt"
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

	// -- Telegram / Payments (Phase 2) --
	TelegramBotToken    string `env:"TELEGRAM_BOT_TOKEN"`
	TelegramBotUsername string `env:"TELEGRAM_BOT_USERNAME"`
	SepayWebhookToken   string `env:"SEPAY_WEBHOOK_TOKEN"`

	// -- SePay QR + Webhook (Phase 05-06) --
	// SepayBankCode is the SePay bank identifier used in QR URL generation (e.g. "MBBank").
	SepayBankCode string `env:"SEPAY_BANK_CODE" envDefault:"MBBank"`
	// SepayBankAccount is the destination bank account number. Never logged in full.
	SepayBankAccount string `env:"SEPAY_BANK_ACCOUNT"`

	// -- External APIs (Phase 4-6) --
	SerpapiKey          string  `env:"SERPAPI_KEY"`
	MozAccessID         string  `env:"MOZ_ACCESS_ID"`
	MozSecret           string  `env:"MOZ_SECRET"`
	TwocaptchaMasterKey string  `env:"TWOCAPTCHA_MASTER_KEY"`
	CapsolverMasterKey  string  `env:"CAPSOLVER_MASTER_KEY"`
	ResendKey           string  `env:"RESEND_KEY"`
	JWTSecret           string  `env:"JWT_SECRET"`
	AdminTelegramIDs    []int64 `env:"ADMIN_TELEGRAM_IDS" envSeparator:","`
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

	return cfg, nil
}

// IsProduction returns true when running in production mode.
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
