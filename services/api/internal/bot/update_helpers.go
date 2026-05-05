// Package bot — update extraction helpers and context key types.
// Kept separate so middleware.go stays under 200 lines.
package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// ctxKey is the unexported type for context value keys in the bot package.
type ctxKey int

const (
	ctxKeyUser ctxKey = iota
	ctxKeyLang
)

// BotUser is a minimal user record attached to handler contexts by loadUser middleware.
// TelegramID is intentionally omitted (YAGNI/M2): re-extract via updateTelegramID(update) when needed.
type BotUser struct {
	ID         uuid.UUID
	Language   string
	IsBanned   bool
	IsVerified bool // Phase 02: needed by /start to decide welcome vs repeat flow.
}

// HandlerFunc is the signature every bot command handler must satisfy.
type HandlerFunc func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error

// UserFromCtx extracts the BotUser attached by loadUser middleware.
// Returns (zero, false) when pool is nil (dev mode without DB).
func UserFromCtx(ctx context.Context) (BotUser, bool) {
	u, ok := ctx.Value(ctxKeyUser).(BotUser)
	return u, ok
}

// LangFromCtx extracts the language code set by i18n middleware. Defaults to "vi".
func LangFromCtx(ctx context.Context) string {
	if lang, ok := ctx.Value(ctxKeyLang).(string); ok && lang != "" {
		return lang
	}
	return "vi"
}

// replyText sends a plain-text message to the update's chat. Errors are swallowed
// because the caller may be inside a recovery path where failing loudly is worse.
func replyText(bot *tgbotapi.BotAPI, update tgbotapi.Update, text string) {
	chatID := updateChatID(update)
	if chatID == 0 {
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	_, _ = bot.Send(msg)
}

// updateChatID extracts the chat ID regardless of update type.
// CallbackQuery.Message is nil-checked: stale/inaccessible messages may omit it (H2 fix).
func updateChatID(u tgbotapi.Update) int64 {
	if u.Message != nil {
		return u.Message.Chat.ID
	}
	if u.CallbackQuery != nil && u.CallbackQuery.Message != nil {
		return u.CallbackQuery.Message.Chat.ID
	}
	return 0
}

// updateTelegramID extracts the sender's Telegram user ID.
func updateTelegramID(u tgbotapi.Update) int64 {
	if u.Message != nil && u.Message.From != nil {
		return u.Message.From.ID
	}
	if u.CallbackQuery != nil && u.CallbackQuery.From.ID != 0 {
		return u.CallbackQuery.From.ID
	}
	return 0
}

// updateCommand extracts the command string (without leading slash) or empty string.
func updateCommand(u tgbotapi.Update) string {
	if u.Message != nil {
		return u.Message.Command()
	}
	return ""
}
