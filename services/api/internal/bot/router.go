// Package bot — command router. Dispatches Telegram updates to handlers.
// Phase 01 ships /ping only; other commands return informational stubs.
package bot

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const helpText = `Snake Backlink Forge — lệnh khả dụng:

/start   — Kích hoạt tài khoản (sắp ra mắt)
/ping    — Kiểm tra kết nối
/topup   — Nạp credits (sắp ra mắt)
/balance — Xem số dư (sắp ra mắt)
/buy     — Mua gói credits (sắp ra mắt)
/history — Lịch sử giao dịch (sắp ra mắt)
/ref     — Chương trình giới thiệu (sắp ra mắt)
/support — Liên hệ hỗ trợ (sắp ra mắt)`

// route dispatches the update to the appropriate handler.
// Returns a HandlerFunc so it can sit at the end of the middleware chain.
func route(b *Bot) HandlerFunc {
	return func(ctx context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
		switch {
		case update.Message != nil && update.Message.IsCommand():
			return dispatchCommand(ctx, api, update)

		case update.CallbackQuery != nil:
			return handleCallback(ctx, api, update)

		case update.Message != nil && update.Message.Contact != nil:
			return handleContact(ctx, api, update)
		}
		// Non-command text messages: ignore silently.
		return nil
	}
}

// dispatchCommand routes /cmd to the appropriate command handler.
func dispatchCommand(ctx context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	cmd := strings.ToLower(update.Message.Command())

	switch cmd {
	case "ping":
		return cmdPing(ctx, api, update)
	case "start":
		return cmdStart(ctx, api, update)
	default:
		return cmdHelp(ctx, api, update)
	}
}

// cmdPing replies "pong" — used for health/latency checks from Telegram.
func cmdPing(_ context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "pong")
	_, err := api.Send(msg)
	return err
}

// cmdStart acknowledges the command; contact-share + trial grant wired in Phase 02.
func cmdStart(_ context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	text := "Chào mừng bạn đến với Snake Backlink Forge!\n\n" +
		"Tính năng đăng ký tài khoản (chia sẻ số điện thoại + nhận trial) sẽ ra mắt sớm.\n\n" +
		helpText
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	_, err := api.Send(msg)
	return err
}

// cmdHelp sends the full command listing.
func cmdHelp(_ context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
	_, err := api.Send(msg)
	return err
}

// handleCallback stubs callback query handling for Phase 02+.
func handleCallback(_ context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	// Guard: CallbackQuery.Message may be nil for stale/inaccessible messages (H2 fix).
	if update.CallbackQuery == nil || update.CallbackQuery.Message == nil {
		return nil
	}

	cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	if _, err := api.Request(cb); err != nil {
		// Answering the callback query is best-effort; don't surface to caller.
		_ = err
	}
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Tính năng này sắp ra mắt.")
	_, err := api.Send(msg)
	return err
}

// handleContact stubs contact-share handling for Phase 02 (/start trial flow).
func handleContact(_ context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Tính năng xác thực số điện thoại sắp ra mắt.")
	_, err := api.Send(msg)
	return err
}
