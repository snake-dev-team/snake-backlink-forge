// Package bot — command router. Dispatches Telegram updates to handlers.
// Phase 02: /start + contact-share flow wired. /key and /regenkey stubs for Phase 03.
package bot

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const helpText = `Snake Backlink Forge — lệnh khả dụng:

/start   — Kích hoạt tài khoản
/ping    — Kiểm tra kết nối
/key     — Xem / tạo API key (sắp ra mắt)
/regenkey — Tái tạo API key (sắp ra mắt)
/topup   — Nạp credits (sắp ra mắt)
/balance — Xem số dư (sắp ra mắt)
/buy     — Mua gói credits (sắp ra mắt)
/history — Lịch sử giao dịch (sắp ra mắt)
/ref     — Chương trình giới thiệu (sắp ra mắt)
/support — Liên hệ hỗ trợ (sắp ra mắt)`

// phase03Stub replies a friendly "coming soon" message for Phase 03+ commands.
const phase03Stub = "Tính năng này đang được nâng cấp. Bot sẽ sớm hỗ trợ lệnh này."

// route dispatches the update to the appropriate handler.
// Returns a HandlerFunc so it can sit at the end of the middleware chain.
func route(b *Bot) HandlerFunc {
	return func(ctx context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
		switch {
		case update.Message != nil && update.Message.IsCommand():
			return dispatchCommand(ctx, b.deps, api, update)

		case update.CallbackQuery != nil:
			return handleCallback(ctx, api, update)

		case update.Message != nil && update.Message.Contact != nil:
			// Route contact-share to HandleStart — FSM gate inside HandleStart checks state.
			if b.deps.UserService != nil {
				return HandleStart(ctx, b.deps, api, update)
			}
			return handleContactFallback(ctx, api, update)
		}
		// Non-command, non-contact messages: ignore silently.
		return nil
	}
}

// dispatchCommand routes /cmd to the appropriate handler.
func dispatchCommand(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	cmd := strings.ToLower(update.Message.Command())

	switch cmd {
	case "ping":
		return cmdPing(ctx, api, update)
	case "start":
		if deps.UserService != nil {
			return HandleStart(ctx, deps, api, update)
		}
		return cmdStartLegacy(ctx, api, update)
	case "key", "regenkey":
		// Phase 03 pending — real handlers wired after KeyService lands.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, phase03Stub)
		_, err := api.Send(msg)
		return err
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

// cmdStartLegacy is the Phase 01 placeholder used when UserService is nil (dev mode).
func cmdStartLegacy(_ context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	text := "Chào mừng bạn đến với Snake Backlink Forge!\n\n" +
		"Tính năng đăng ký tài khoản sẽ ra mắt sớm.\n\n" +
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
		_ = err // answering callback is best-effort
	}
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Tính năng này sắp ra mắt.")
	_, err := api.Send(msg)
	return err
}

// handleContactFallback is used when UserService is nil (dev mode).
func handleContactFallback(_ context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Tính năng xác thực số điện thoại sắp ra mắt.")
	_, err := api.Send(msg)
	return err
}

