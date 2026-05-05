// webhook_deps.go — Phase 06: Deps container for webhook handler + F3 error classifier.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/bot/templates"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// WebhookDeps holds every dependency the SePay webhook handler needs.
// Constructed once in main.go and passed into SePayWebhook().
type WebhookDeps struct {
	Pool         *pgxpool.Pool
	Rdb          *goredis.Client
	Cfg          *config.Config
	Log          *zap.Logger
	WebhookSvc   *service.WebhookService
	AdminAlertCh chan<- notify.AdminAlert
	// BotAPI sends Telegram DMs on payment success. May be nil in tests or when
	// TELEGRAM_BOT_TOKEN is empty; notify is silently skipped in that case.
	BotAPI *tgbotapi.BotAPI
	// Templates renders i18n payment-success messages. May be nil in tests.
	Templates *templates.Renderer
	// RootCtx is the server-lifetime context. Notify goroutines are scoped to it
	// with a 10s timeout so they don't outlive the process on SIGTERM. [H6]
	RootCtx context.Context //nolint:containedctx // intentional: scoping notify goroutines
}

// retryEnvelope is the Redis list payload for lock-contention retries. [F3]
type retryEnvelope struct {
	Payload    sepay.Payload `json:"payload"`
	Attempts   int           `json:"attempts"`
	OriginalTS int64         `json:"original_ts"`
}

// ack200 returns a 200 JSON response in SePay's expected shape.
// success=true tells SePay not to retry; success=false is used for auth/business errors
// to prevent SePay retry storms while still signalling a problem via reason.
func ack200(c *fiber.Ctx, success bool, reason string) error {
	body := fiber.Map{"success": success}
	if reason != "" {
		body["reason"] = reason
	}
	return c.Status(fiber.StatusOK).JSON(body)
}

// classifyWebhookError maps Go errors to HTTP responses per the [F3] policy matrix.
//
//	Lock contention (deadlock/serialization/deadline) → 200 queued_for_retry + Redis enqueue
//	Hard infra (pool exhausted)                       → 503
//	Unknown/panic                                     → 500 + admin alert
func classifyWebhookError(
	c *fiber.Ctx,
	deps *WebhookDeps,
	err error,
	orderCode string,
	p sepay.Payload,
) error {
	var pgErr *pgconn.PgError
	switch {
	// Lock contention: deadlock (40P01) or serialization failure (40001) or deadline.
	case errors.As(err, &pgErr) &&
		(pgErr.Code == "40P01" || pgErr.Code == "40001"),
		isDeadlineErr(err):

		env := retryEnvelope{Payload: p, Attempts: 0, OriginalTS: time.Now().Unix()}
		data, _ := json.Marshal(env)
		// Bounded LPUSH + LTRIM — keeps newest 500 entries. [round-3]
		_ = deps.Rdb.LPush(context.Background(), "sepay_retry_queue", data).Err()
		_ = deps.Rdb.LTrim(context.Background(), "sepay_retry_queue", 0, 499).Err()
		deps.Log.Warn("sepay webhook queued for retry",
			zap.String("order_code", orderCode),
			zap.Error(err),
		)
		return ack200(c, true, "queued_for_retry")

	// Hard infra: connection pool exhausted (pgxpool returns context.DeadlineExceeded or
	// a message containing "acquire" when pool is saturated and ctx times out).
	// Let SePay retry — this is typically a short outage.
	case isPoolExhaustedErr(err):
		deps.Log.Error("sepay webhook: DB pool exhausted", zap.Error(err))
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"reason":  "service_unavailable",
		})

	// Catch-all: audit + alert admin + 500.
	default:
		deps.Log.Error("sepay webhook unknown error",
			zap.String("order_code", orderCode),
			zap.Error(err),
		)
		webhookAuditLog(deps, "sepay_internal_error", map[string]any{
			"order_code": orderCode,
			"err":        err.Error(),
		})
		if deps.AdminAlertCh != nil {
			select {
			case deps.AdminAlertCh <- notify.AdminAlert{
				Kind:      "webhook_internal_error",
				OrderCode: orderCode,
				Err:       err.Error(),
			}:
			default:
			}
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false})
	}
}

// isDeadlineErr returns true for context deadline/cancellation errors.
func isDeadlineErr(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// isPoolExhaustedErr detects pgxpool connection exhaustion.
// pgxpool v5 does not export a typed sentinel; exhaustion manifests as a context deadline
// wrapping "failed to acquire connection" when all connections are in use and ctx times out.
func isPoolExhaustedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "failed to acquire") ||
		strings.Contains(msg, "acquire") && strings.Contains(msg, "pool")
}

// webhookAuditLog inserts an audit_log row via the pool directly (best-effort).
// [M3] Uses RootCtx (with a 2s per-insert timeout) so audit writes drain on SIGTERM
// alongside other consumers instead of outliving the process with context.Background().
func webhookAuditLog(deps *WebhookDeps, event string, meta map[string]any) {
	if deps.Pool == nil {
		return
	}
	ctx := deps.RootCtx
	if ctx == nil {
		ctx = context.Background() // fallback for tests that don't wire RootCtx
	}
	insertCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	metaJSON, _ := json.Marshal(meta)
	if _, err := deps.Pool.Exec(insertCtx,
		`INSERT INTO audit_log (event, metadata) VALUES ($1, $2)`,
		event, metaJSON,
	); err != nil {
		if deps.Log != nil {
			deps.Log.Warn("audit log insert failed", zap.String("event", event), zap.Error(err))
		}
	}
}
