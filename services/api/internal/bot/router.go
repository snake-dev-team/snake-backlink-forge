// Package bot — command router. Dispatches Telegram updates to handlers.
// Phase 03: /key and /regenkey wired. Callback dispatch table extended.
package bot

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const helpText = `Snake Backlink Forge — lệnh khả dụng:

/start    — Kích hoạt tài khoản
/ping     — Kiểm tra kết nối
/key      — Xem API key (masked) + nút tạo lại
/regenkey — Tạo lại API key (key cũ bị thu hồi)
/topup    — Nạp credits (sắp ra mắt)
/balance  — Xem số dư (sắp ra mắt)
/buy      — Mua gói credits (sắp ra mắt)
/history  — Lịch sử giao dịch (sắp ra mắt)
/ref      — Chương trình giới thiệu (sắp ra mắt)
/support  — Liên hệ hỗ trợ (sắp ra mắt)`

// route dispatches the update to the appropriate handler.
// Returns a HandlerFunc so it can sit at the end of the middleware chain.
func route(b *Bot) HandlerFunc {
	return func(ctx context.Context, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
		switch {
		case update.Message != nil && update.Message.IsCommand():
			return dispatchCommand(ctx, b.deps, api, update)

		case update.CallbackQuery != nil:
			return dispatchCallback(ctx, b.deps, api, update)

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
	case "key":
		return HandleKey(ctx, deps, api, update)
	case "regenkey":
		return HandleRegenKey(ctx, deps, api, update)
	case "balance":
		return HandleBalance(ctx, deps, api, update)
	case "buy":
		return HandleBuy(ctx, deps, api, update)
	case "topup":
		return HandleTopup(ctx, deps, api, update)
	default:
		return cmdHelp(ctx, api, update)
	}
}

// dispatchCallback routes inline keyboard callback queries by their data string.
// All handlers are guarded via guardCallback: stale callbacks (nil Message) are
// answered with an expiry notice and discarded before reaching handler logic (H1).
func dispatchCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}

	// H1: guard before dereferencing CallbackQuery.Message in any handler.
	// key:regen:confirm and key:regen:cancel need Message for chat ID; guard all uniformly.
	if update.CallbackQuery.Message == nil {
		return guardCallback(api, update)
	}

	data := update.CallbackQuery.Data

	switch {
	case data == "key:copy_prefix":
		return handleKeyCopyPrefixCallback(ctx, deps, api, update)
	case data == "key:regen":
		// "Regenerate" button on /key view — dispatch to /regenkey flow.
		return HandleRegenKey(ctx, deps, api, update)
	case data == "key:regen:confirm":
		return handleRegenConfirmCallback(ctx, deps, api, update)
	case data == "key:regen:cancel":
		return handleRegenCancelCallback(ctx, deps, api, update)

	// Phase 05: buy flow callbacks.
	// buy:pkg:<code> — package selected from menu.
	case strings.HasPrefix(data, "buy:pkg:"):
		return handleBuyPackageCallback(ctx, deps, api, update)
	// buy:confirm:<code> — user confirmed purchase.
	case strings.HasPrefix(data, "buy:confirm:"):
		return handleBuyConfirmCallback(ctx, deps, api, update)
	// buy:cancel — user cancelled buy flow.
	case data == "buy:cancel":
		return handleBuyCancelCallback(ctx, deps, api, update)

	// Phase 05: topup flow callbacks.
	// topup:check:<tx_id> — poll payment status.
	case strings.HasPrefix(data, "topup:check:"):
		return handleTopupCheckCallback(ctx, deps, api, update)
	// topup:cancel:<tx_id> — cancel pending transaction.
	case strings.HasPrefix(data, "topup:cancel:"):
		return handleTopupCancelCallback(ctx, deps, api, update)

	default:
		return handleUnknownCallback(api, update)
	}
}

// guardCallback answers a stale/inaccessible callback query (Message == nil) so
// Telegram removes the loading spinner, then returns nil (not an error — caller discards).
// All 4 callback handlers (key:copy_prefix, key:regen, key:regen:confirm, key:regen:cancel)
// rely on this single guard placed in dispatchCallback before the switch (H1 fix).
func guardCallback(api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "Tin nhắn đã hết hạn.")
	_, _ = api.Request(cb) // best-effort: ack is fire-and-forget
	return nil
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

// handleUnknownCallback answers unknown callback queries with an empty ack
// so Telegram removes the loading spinner, then sends a generic reply.
func handleUnknownCallback(api *tgbotapi.BotAPI, update tgbotapi.Update) error {
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

