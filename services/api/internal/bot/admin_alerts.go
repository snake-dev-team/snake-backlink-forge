// admin_alerts.go — Phase 06 [Q5]: admin DM consumer + non-blocking send helper.
// Phase 08: adds AuditFailAlertWatcher goroutine for sepay_auth_fail burst detection.
// AdminAlert struct lives in internal/notify to avoid service→bot import cycle.
package bot

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"go.uber.org/zap"
)

// SendAdminAlertNonBlocking attempts a non-blocking send to ch.
// If ch is full (backpressure), the alert is dropped and a Warn is logged.
// Never blocks the caller — webhook hot path must not stall on admin DM failures.
func SendAdminAlertNonBlocking(ch chan<- notify.AdminAlert, alert notify.AdminAlert, log *zap.Logger) {
	if alert.At.IsZero() {
		alert.At = time.Now()
	}
	select {
	case ch <- alert:
	default:
		log.Warn("admin alert channel full, dropping alert",
			zap.String("kind", alert.Kind),
			zap.String("order_code", alert.OrderCode),
		)
	}
}

// ConsumeAdminAlerts is a supervisor that calls runAdminAlertsOnce in a recover wrapper.
// If the inner consumer panics (e.g. nil BotAPI, encoding error), the supervisor logs it
// and restarts. Exits only when ctx is cancelled or ch is closed.
// [M1] Spec risk-table line 580: consumer goroutine must recover from panic + restart loop.
func ConsumeAdminAlerts(
	ctx context.Context,
	ch <-chan notify.AdminAlert,
	api *tgbotapi.BotAPI,
	cfg *config.Config,
	log *zap.Logger,
) {
	for {
		if !runAdminAlertsOnce(ctx, ch, api, cfg, log) {
			return // ctx cancelled or channel closed — clean exit
		}
		// runAdminAlertsOnce returned true only on panic-recover; loop restarts.
	}
}

// runAdminAlertsOnce runs the alert-drain select loop, recovering from panics.
// Returns false when ctx is cancelled or ch is closed (caller should stop).
// Returns true when a panic was recovered (caller should restart).
func runAdminAlertsOnce(
	ctx context.Context,
	ch <-chan notify.AdminAlert,
	api *tgbotapi.BotAPI,
	cfg *config.Config,
	log *zap.Logger,
) (continueLoop bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("admin alert consumer: panic recovered — restarting loop",
				zap.Any("panic", r),
			)
			continueLoop = true
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return false
		case alert, ok := <-ch:
			if !ok {
				// Channel closed by main.go shutdown — drain complete.
				return false
			}
			text := formatAlert(alert)
			for _, adminID := range cfg.AdminTelegramIDs {
				msg := tgbotapi.NewMessage(adminID, text)
				if _, err := api.Send(msg); err != nil {
					log.Warn("admin alert DM failed",
						zap.Int64("admin_tg_id", adminID),
						zap.String("kind", alert.Kind),
						zap.Error(err),
					)
				}
			}
		}
	}
}

// formatAlert renders an AdminAlert as a human-readable Telegram message.
// Uses plain text (no Markdown) to avoid parse errors from unescaped special chars in err strings.
func formatAlert(a notify.AdminAlert) string {
	ts := a.At.Format("2006-01-02 15:04:05 MST")
	switch a.Kind {
	case "overpaid":
		return fmt.Sprintf(
			"[ALERT] Over-payment detected\nOrder: %s\nUser: %s\nExcess: %s VND\nBonus credits granted: %d\nAt: %s",
			a.OrderCode, a.UserID, formatAlertVND(a.ExcessVND), a.BonusCredits, ts,
		)
	case "auth_fail_burst":
		return fmt.Sprintf(
			"[ALERT] SePay auth-fail burst\nDetails: %s\nAt: %s\n\nCheck SEPAY_WEBHOOK_TOKEN rotation.",
			a.Err, ts,
		)
	case "webhook_internal_error":
		return fmt.Sprintf(
			"[ALERT] Webhook internal error\nOrder: %s\nError: %s\nAt: %s",
			a.OrderCode, a.Err, ts,
		)
	case "retry_dead_letter":
		return fmt.Sprintf(
			"[ALERT] Retry exhausted → dead-letter\nOrder: %s\nError: %s\nAt: %s\n\nInspect sepay_dead_letter Redis list.",
			a.OrderCode, a.Err, ts,
		)
	case "retry_queue_backlog":
		return fmt.Sprintf(
			"[ALERT] Retry queue backlog\nDetails: %s\nAt: %s\n\nMonitor sepay_retry_queue length.",
			a.Err, ts,
		)
	default:
		return fmt.Sprintf("[ALERT] %s\nOrder: %s\nError: %s\nAt: %s", a.Kind, a.OrderCode, a.Err, ts)
	}
}

// formatAlertVND formats a VND integer as a comma-separated string (e.g. 71000 → "71,000").
// Named formatAlertVND to avoid collision with bot/cmd_balance.go's formatVND.
func formatAlertVND(n int64) string {
	s := fmt.Sprintf("%d", n)
	// Insert commas every 3 digits from the right.
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, ch := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}

// AuditFailAlertWatcher polls audit_log every 60 seconds for sepay_auth_fail bursts.
// Threshold: ≥20 events in the last 15 minutes → fires one AdminAlert{Kind:"auth_fail_burst"}.
// Window-bucket dedup (now.Unix() / 900) prevents duplicate alerts within the same 15-min window.
// GC retains the last 4 buckets to guard against clock skew.
// Spawned in cmd/api/main.go bound to rootCtx:
//
//	go bot.AuditFailAlertWatcher(rootCtx, dbPool, adminAlertCh, log.Named("audit_fail_watcher"))
func AuditFailAlertWatcher(ctx context.Context, pool *pgxpool.Pool, alertCh chan<- notify.AdminAlert, log *zap.Logger) {
	auditFailAlertWatcherWithTicker(ctx, pool, alertCh, log, 60*time.Second)
}

// auditFailAlertWatcherWithTicker is the testable inner implementation.
// Exported AuditFailAlertWatcher calls this with the production 60s interval.
// Tests inject a shorter interval (e.g. 1s) for fast assertion.
func auditFailAlertWatcherWithTicker(
	ctx context.Context,
	pool *pgxpool.Pool,
	alertCh chan<- notify.AdminAlert,
	log *zap.Logger,
	tickInterval time.Duration,
) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	alertedWindows := make(map[int64]struct{})

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			bucket := now.Unix() / (15 * 60) // 15-min window bucket
			if _, already := alertedWindows[bucket]; already {
				continue
			}

			var count int
			err := pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM audit_log WHERE event='sepay_auth_fail' AND created_at > NOW() - INTERVAL '15 minutes'`,
			).Scan(&count)
			if err != nil {
				log.Warn("audit_fail_watcher: poll failed", zap.Error(err))
				continue
			}

			if count >= 20 {
				SendAdminAlertNonBlocking(alertCh, notify.AdminAlert{
					Kind: "auth_fail_burst",
					Err:  fmt.Sprintf("%d SePay webhook auth failures in 15min", count),
					At:   now,
				}, log)
				alertedWindows[bucket] = struct{}{}
				// GC old buckets — retain [bucket-4, bucket] to tolerate minor clock jitter.
				for b := range alertedWindows {
					if b < bucket-4 {
						delete(alertedWindows, b)
					}
				}
			}
		}
	}
}
