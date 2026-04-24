// cmd_balance.go — Phase 04: /balance command handler.
// Reads wallet via WalletService.GetBalance, formats credits + VND spent, replies to user.
package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// HandleBalance handles the /balance command.
// Requires loadUser middleware (Phase 01) to have attached a BotUser to ctx.
// If WalletService is nil (dev mode), replies with a graceful "coming soon" message.
func HandleBalance(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	if deps.WalletService == nil {
		msg := tgbotapi.NewMessage(chatID, "Tính năng xem số dư sắp ra mắt.")
		_, err := api.Send(msg)
		return err
	}

	user, ok := UserFromCtx(ctx)
	if !ok || user.ID.String() == "00000000-0000-0000-0000-000000000000" {
		// Synthetic dev-mode user or missing user — wallet not yet created.
		msg := tgbotapi.NewMessage(chatID, "⚠️ Không tìm thấy ví. /start lại để tạo ví.")
		_, err := api.Send(msg)
		return err
	}

	wallet, err := deps.WalletService.GetBalance(ctx, user.ID)
	if err != nil {
		deps.Log.Error("HandleBalance: GetBalance failed",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
		msg := tgbotapi.NewMessage(chatID, "⚠️ Không tìm thấy ví. /start lại để tạo ví.")
		_, err2 := api.Send(msg)
		if err2 != nil {
			return fmt.Errorf("HandleBalance: send error reply: %w", err2)
		}
		return nil
	}

	// VND thousand-separator: use comma (English convention, widely used in VN fintech UIs).
	// Vietnamese locale uses '.' as thousands separator, but ',' is ubiquitous in digital products.
	// Adjust formatVND if the product preference changes.
	text := fmt.Sprintf(
		"💰 *Số dư của bạn:*\n\n"+
			"🔥 Premium: *%d* credit\n"+
			"⚡ Standard: *%d* credit\n\n"+
			"💸 Đã chi: %sđ",
		wallet.PremiumCredits,
		wallet.StandardCredits,
		formatVND(wallet.TotalVndSpent),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}

// formatVND formats an int64 as a comma-separated string (e.g. 1329000 → "1,329,000").
// Zero renders as "0". Negative values retain the minus prefix.
// Convention: comma `,` as thousands separator (English-style, common in VN fintech UIs).
func formatVND(n int64) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	s := strconv.FormatInt(n, 10)
	var b strings.Builder

	// Write leading digits (len(s) % 3 chars) before the first comma group.
	start := len(s) % 3
	if start > 0 {
		b.WriteString(s[:start])
		if len(s) > start {
			b.WriteByte(',')
		}
	}
	for i := start; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}

	if neg {
		return "-" + b.String()
	}
	return b.String()
}
