// cmd_language.go — Phase 07: /language command + lang:set:<vi|en> callback.
// Persists language preference to users.language via raw pgx (no new sqlc query needed).
package bot

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// validLanguages contains the accepted language codes.
var validLanguages = map[string]bool{"vi": true, "en": true}

// HandleLanguage handles the /language command — shows language selection keyboard.
func HandleLanguage(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇻🇳 Tiếng Việt", "lang:set:vi"),
			tgbotapi.NewInlineKeyboardButtonData("🇬🇧 English", "lang:set:en"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "🌐 Chọn ngôn ngữ / Choose language:")
	msg.ReplyMarkup = keyboard
	_, err := api.Send(msg)
	return err
}

// HandleLangSetCallback handles "lang:set:<vi|en>" — persists language and confirms.
func HandleLangSetCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	lang := strings.TrimPrefix(update.CallbackQuery.Data, "lang:set:")
	if !validLanguages[lang] {
		deps.Log.Warn("HandleLangSetCallback: invalid lang", zap.String("lang", lang))
		return nil
	}

	user, ok := UserFromCtx(ctx)
	if !ok {
		return nil
	}

	// Persist via raw pgx — no new sqlc query needed (YAGNI).
	if deps.Pool != nil {
		_, err := deps.Pool.Exec(ctx,
			`UPDATE users SET language = $1, updated_at = NOW() WHERE id = $2`,
			lang, user.ID,
		)
		if err != nil {
			deps.Log.Error("HandleLangSetCallback: UPDATE users language failed",
				zap.String("user_id", user.ID.String()),
				zap.String("lang", lang),
				zap.Error(err),
			)
			// Non-fatal: still confirm to user (optimistic).
		}
	}

	var confirmText string
	switch lang {
	case "vi":
		confirmText = "✅ Đã đổi sang Tiếng Việt."
	case "en":
		confirmText = "✅ Switched to English."
	}

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			confirmText,
		)
		if _, err := api.Send(edit); err != nil {
			deps.Log.Debug("HandleLangSetCallback: edit message failed", zap.Error(err))
		}
	}
	return nil
}
