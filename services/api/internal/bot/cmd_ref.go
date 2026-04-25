// cmd_ref.go — Phase 07: /ref command.
// Shows user's referral code + deep-link + total referred count.
// EnsureCode is idempotent — safe to call on every /ref invocation.
package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// HandleRef handles the /ref command.
func HandleRef(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	user, ok := UserFromCtx(ctx)
	if !ok || user.ID.String() == "00000000-0000-0000-0000-000000000000" {
		replyText(api, update, "⚠️ Vui lòng /start để khởi tạo tài khoản.")
		return nil
	}

	if deps.RefService == nil {
		replyText(api, update, "Tính năng giới thiệu sắp ra mắt.")
		return nil
	}

	code, err := deps.RefService.EnsureCode(ctx, user.ID)
	if err != nil {
		deps.Log.Error("HandleRef: EnsureCode failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		replyText(api, update, "⚠️ Không thể tải mã giới thiệu. Vui lòng thử lại sau.")
		return nil
	}

	totalReferred, err := deps.RefService.GetTotalReferred(ctx, user.ID)
	if err != nil {
		deps.Log.Warn("HandleRef: GetTotalReferred failed (non-fatal)",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		// Non-fatal: display 0 rather than failing the command.
	}

	botUsername := ""
	if deps.Cfg != nil {
		botUsername = deps.Cfg.TelegramBotUsername
	}

	var deepLink string
	if botUsername != "" {
		deepLink = fmt.Sprintf("t.me/%s?start=ref_%s", botUsername, code)
	} else {
		deepLink = fmt.Sprintf("ref_%s (cấu hình TELEGRAM_BOT_USERNAME để có deep-link)", code)
	}

	text := fmt.Sprintf(
		"🎁 <b>Mã giới thiệu của bạn:</b> <code>%s</code>\n\n"+
			"🔗 Link: %s\n\n"+
			"👥 Đã giới thiệu: <b>%d</b> người",
		code, deepLink, totalReferred,
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err = api.Send(msg)
	return err
}
