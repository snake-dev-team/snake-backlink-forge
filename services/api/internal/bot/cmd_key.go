// cmd_key.go — /key command handler.
// Shows the user's active API key in masked form with inline keyboard actions.
// Plaintext is NEVER shown here — it was shown once on issue (/start or /regenkey confirm).
package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// HandleKey handles the /key command.
// Flow:
//  1. Load user from ctx (set by loadUser middleware).
//  2. Call KeyService.GetActiveMasked.
//  3. If key exists: reply masked display + inline keyboard [Copy prefix | Regenerate].
//  4. If no key: reply instruction to /start (edge case — Phase 03 wires real KeyService
//     so /start always issues a key; this path covers users who joined before Phase 03).
func HandleKey(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)

	user, ok := UserFromCtx(ctx)
	if !ok || user.ID.String() == "00000000-0000-0000-0000-000000000000" {
		replyText(api, update, "Vui lòng dùng /start để khởi tạo tài khoản trước.")
		return nil
	}

	if deps.KeyService == nil {
		replyText(api, update, "Hệ thống key đang khởi động. Vui lòng thử lại sau.")
		return nil
	}

	masked, exists, err := deps.KeyService.GetActiveMasked(ctx, user.ID)
	if err != nil {
		deps.Log.Error("HandleKey: GetActiveMasked failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		replyText(api, update, "⚠️ Hệ thống gặp sự cố. Vui lòng thử lại sau.")
		return err
	}

	if !exists {
		msg := tgbotapi.NewMessage(chatID,
			"⚠️ Bạn chưa có API key. Hãy /start lại để nhận key.")
		_, err = api.Send(msg)
		return err
	}

	// Build inline keyboard: Copy prefix + Regenerate.
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Copy prefix", "key:copy_prefix"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Regenerate", "key:regen"),
		),
	)

	// Note: KeyKeyShow uses Markdown (the bundle wraps {{.KeyPrefixMasked}} in
	// backticks). Switch ParseMode to Markdown so the code-block renders.
	// HTML escaping is no longer needed; the masked prefix is alphanumeric+•.
	text := renderTplCtx(ctx, deps, tplKeyShow, struct {
		KeyPrefixMasked string
	}{
		KeyPrefixMasked: masked,
	})

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	_, err = api.Send(msg)
	return err
}

// handleKeyCopyPrefixCallback handles the "Copy prefix" inline button.
// Answers the callback query with the key prefix as a popup notification.
// The prefix is already the stored key_prefix (12 chars, e.g. "sbf_live_Zk3").
func handleKeyCopyPrefixCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}

	user, ok := UserFromCtx(ctx)
	if !ok || deps.KeyService == nil {
		cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		_, _ = api.Request(cb)
		return nil
	}

	masked, exists, err := deps.KeyService.GetActiveMasked(ctx, user.ID)
	if err != nil || !exists {
		cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "Key không tồn tại.")
		_, _ = api.Request(cb)
		return err
	}

	// Extract prefix from masked: everything before the bullet separator.
	prefix := masked
	for i, ch := range masked {
		if ch == '•' {
			prefix = masked[:i]
			break
		}
	}

	// AnswerCallbackQuery with show_alert=true so the prefix pops up for the user to copy.
	cb := tgbotapi.NewCallbackWithAlert(update.CallbackQuery.ID, prefix)
	_, err = api.Request(cb)
	return err
}
