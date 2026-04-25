// webhook_notify.go — Phase 06+10: real Telegram DM on SePay payment success.
//
// Replaces the Phase 06 placeholder (line 132 of webhook.go original) with a
// goroutine that:
//   1. Looks up user telegram_id + language from ProcessResult.UserID
//   2. Picks template by branch (recovered/overpaid/standard)
//   3. Renders via templates.Renderer + sends via tgbotapi
//
// Failure modes (all logged, never panic, never retry):
//   - User row missing → log Warn (should not happen — UUID came from same DB)
//   - BotAPI nil      → silently skip (test or bot-disabled deploy)
//   - Templates nil   → silently skip (likewise)
//   - Telegram send err→ log Warn (user can /balance to verify credits)
package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/bot/templates"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

const notifyTimeout = 5 * time.Second

// notifyTelegramSuccess fires the post-commit success DM in a non-blocking goroutine.
// Returns immediately; the goroutine is bounded by deps.RootCtx + notifyTimeout.
//
// Caller invariant: deps.RootCtx must be non-nil.
// Result invariant: result.UserID must be a real users.id (CAS UPDATE returned it).
func notifyTelegramSuccess(deps *WebhookDeps, result service.ProcessResult) {
	if deps.BotAPI == nil || deps.Templates == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(deps.RootCtx, notifyTimeout)
		defer cancel()
		sendNotify(ctx, deps, result)
	}()
}

// sendNotify is the synchronous body of the notify goroutine.
// Split from notifyTelegramSuccess for direct testability.
func sendNotify(ctx context.Context, deps *WebhookDeps, result service.ProcessResult) {
	tgID, lang, err := loadUserNotifyInfo(ctx, deps, result.UserID)
	if err != nil {
		deps.Log.Warn("notify: user lookup failed",
			zap.String("user_id", result.UserID.String()),
			zap.Error(err),
		)
		return
	}

	pkg, ok := service.Packages[result.PackageCode]
	if !ok {
		deps.Log.Warn("notify: unknown package code",
			zap.String("package_code", result.PackageCode),
		)
		return
	}

	key, data := pickTemplate(result, pkg, lang)
	text := deps.Templates.Render(lang, key, data)

	msg := tgbotapi.NewMessage(tgID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := deps.BotAPI.Send(msg); err != nil {
		deps.Log.Warn("notify: telegram send failed",
			zap.Int64("tg_id", tgID),
			zap.String("template_key", key),
			zap.Error(err),
		)
	}
}

// loadUserNotifyInfo fetches telegram_id + language for the given user UUID.
func loadUserNotifyInfo(ctx context.Context, deps *WebhookDeps, userID uuid.UUID) (int64, string, error) {
	if deps.Pool == nil {
		return 0, "", fmt.Errorf("notify: deps.Pool nil")
	}
	var tgID int64
	var lang string
	err := deps.Pool.QueryRow(ctx,
		`SELECT telegram_id, language FROM users WHERE id = $1`,
		userID,
	).Scan(&tgID, &lang)
	if err != nil {
		return 0, "", err
	}
	if lang == "" {
		lang = "vi" // default to VN if unset
	}
	return tgID, lang, nil
}

// pickTemplate returns (key, data) for the given result. Three branches:
//
//	WasCancelled → KeyTopupRecovered (Q2 cancel-then-pay recovery message)
//	Overpaid     → KeyTopupOverpaidOK (base + bonus message)
//	default      → KeyTopupPaidOK (standard success)
//
// Display name selection: lang=="en" uses pkg.DisplayEN, else pkg.DisplayVI.
func pickTemplate(result service.ProcessResult, pkg service.Package, lang string) (string, any) {
	display := pkg.DisplayVI
	if lang == "en" {
		display = pkg.DisplayEN
	}
	creditSummary := buildCreditSummary(pkg)

	switch {
	case result.Overpaid && result.BonusCredits > 0:
		return templates.KeyTopupOverpaidOK, struct {
			Display            string
			CreditSummary      string
			ExcessVNDFormatted string
			BonusCredits       int
			BonusPool          string
		}{
			Display:            display,
			CreditSummary:      creditSummary,
			ExcessVNDFormatted: formatVNDComma(result.ExcessVND),
			BonusCredits:       result.BonusCredits,
			BonusPool:          result.BonusPool,
		}
	case result.WasCancelled:
		return templates.KeyTopupRecovered, struct {
			Display       string
			CreditSummary string
		}{Display: display, CreditSummary: creditSummary}
	default:
		return templates.KeyTopupPaidOK, struct {
			Display       string
			CreditSummary string
		}{Display: display, CreditSummary: creditSummary}
	}
}

// buildCreditSummary mirrors bot/cmd_buy.go:124. Duplicated here to avoid a
// handlers→bot import cycle (bot depends on handlers indirectly via main.go wiring).
func buildCreditSummary(pkg service.Package) string {
	switch {
	case pkg.PremiumCredits > 0 && pkg.StandardCredits > 0:
		return fmt.Sprintf("%d Premium + %d Standard", pkg.PremiumCredits, pkg.StandardCredits)
	case pkg.PremiumCredits > 0:
		return fmt.Sprintf("%d Premium", pkg.PremiumCredits)
	default:
		return fmt.Sprintf("%d Standard", pkg.StandardCredits)
	}
}

// formatVNDComma renders e.g. 71000 → "71,000". Comma separator matches
// bot/cmd_balance.go:formatVND for UI consistency across bot and webhook DMs.
func formatVNDComma(n int64) string {
	s := strconv.FormatInt(n, 10)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}
