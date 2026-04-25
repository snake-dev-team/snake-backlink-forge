// admin_alerts.go — Phase 06 [Q5]: admin DM consumer + non-blocking send helper.
// AdminAlert struct lives in internal/notify to avoid service→bot import cycle.
package bot

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
