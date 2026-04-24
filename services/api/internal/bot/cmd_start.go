// Package bot — /start command + contact-share handler.
// Phase 02: implements user upsert, FSM awaiting_contact, phone verify, trial grant.
package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

const (
	// stateAwaitingContact is set on FSM after sending the contact-request keyboard.
	stateAwaitingContact = "awaiting_contact"
	// contactStateTTL is how long the bot waits for a contact share before the state expires.
	contactStateTTL = 10 * time.Minute
)

// HandleStart handles /start command and the full multi-step contact-share flow.
// Route table (bot/router.go) delegates here for:
//   - update.Message.IsCommand() && cmd=="start"  → first time or repeat
//   - update.Message.Contact != nil                → contact received (FSM gate checked here)
func HandleStart(ctx context.Context, deps *Deps, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	// Contact-share event is dispatched to HandleStart from router.go when FSM=awaiting_contact.
	if update.Message != nil && update.Message.Contact != nil {
		return handleContactShare(ctx, deps, bot, update)
	}
	return handleStartCommand(ctx, deps, bot, update)
}

// handleStartCommand processes the /start command.
func handleStartCommand(ctx context.Context, deps *Deps, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	from := update.Message.From
	if from == nil {
		return nil // anonymous channel message — ignore
	}

	// Upsert user stub + ensure wallet row.
	user, err := deps.UserService.EnsureStub(ctx, from.ID, from.UserName, from.FirstName)
	if err != nil {
		deps.Log.Error("HandleStart: EnsureStub failed",
			zap.Int64("tg_id", from.ID), zap.Error(err))
		replyText(bot, update, "Đã xảy ra lỗi khi khởi tạo tài khoản. Vui lòng thử lại.")
		return err
	}

	// Attach full user to context for downstream (also updates BotUser in ctx).
	botUser := BotUser{
		ID:         user.ID,
		Language:   user.Language,
		IsBanned:   user.IsBanned,
		IsVerified: user.IsVerified,
	}
	_ = botUser // used by FSM state; kept for future middleware handoff

	tgID := from.ID
	stateStore := NewStore(deps.Rdb)

	if user.IsVerified {
		// Already verified: clear FSM + show repeat welcome.
		_ = stateStore.Clear(ctx, tgID)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			"Tài khoản của bạn đã được xác thực.\n\n"+
				"Dùng /key để xem API key và /balance để kiểm tra số dư.")
		_, err := bot.Send(msg)
		return err
	}

	// First-time flow: request contact share.
	if storeErr := stateStore.Save(ctx, tgID, State{Name: stateAwaitingContact}, contactStateTTL); storeErr != nil {
		deps.Log.Warn("HandleStart: failed to save FSM state",
			zap.Int64("tg_id", tgID), zap.Error(storeErr))
		// Non-fatal: continue sending the keyboard even without FSM persistence.
	}

	contactBtn := tgbotapi.NewKeyboardButtonContact("Chia sẻ số điện thoại")
	keyboard := tgbotapi.NewReplyKeyboard([]tgbotapi.KeyboardButton{contactBtn})
	keyboard.OneTimeKeyboard = true
	keyboard.ResizeKeyboard = true

	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		"Chào mừng bạn đến với Snake Backlink Forge!\n\n"+
			"Để kích hoạt tài khoản và nhận <b>5 Standard credits miễn phí</b>, "+
			"vui lòng nhấn nút bên dưới để chia sẻ số điện thoại.")
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard

	_, err = bot.Send(msg)
	return err
}

// handleContactShare processes the Telegram contact-share event.
// Guards: FSM must be awaiting_contact AND contact.UserID must match from.ID.
func handleContactShare(ctx context.Context, deps *Deps, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	from := update.Message.From
	contact := update.Message.Contact
	if from == nil || contact == nil {
		return nil
	}

	tgID := from.ID

	// Security: reject spoofed contacts (user sharing someone else's number).
	if contact.UserID != 0 && contact.UserID != tgID {
		replyText(bot, update, "Chỉ được chia sẻ số điện thoại của chính bạn.")
		return nil
	}

	stateStore := NewStore(deps.Rdb)

	// FSM gate: only process contact if state=awaiting_contact.
	st, err := stateStore.Load(ctx, tgID)
	if err != nil || st.Name != stateAwaitingContact {
		// Stale contact share or wrong state — ignore silently.
		return nil
	}

	// Retrieve user from ctx (set by loadUser middleware).
	botUser, ok := UserFromCtx(ctx)
	if !ok || botUser.ID.String() == "" {
		deps.Log.Error("handleContactShare: no user in ctx", zap.Int64("tg_id", tgID))
		replyText(bot, update, "Đã xảy ra lỗi. Vui lòng thử lại /start.")
		return fmt.Errorf("handleContactShare: no BotUser in context for tg_id=%d", tgID)
	}

	// Call atomic trial gate.
	plaintext, prefix, trialErr := deps.UserService.VerifyContactAndGrantTrial(ctx, botUser.ID, contact.PhoneNumber)

	// Clear FSM state regardless of outcome.
	_ = stateStore.Clear(ctx, tgID)

	if trialErr != nil {
		return handleTrialError(bot, update, trialErr)
	}

	// Remove the contact keyboard (send RemoveKeyboard).
	removeKbd := tgbotapi.NewRemoveKeyboard(true)

	if plaintext == "" {
		// noopKeyIssuer — key not yet wired (Phase 03 pending).
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			"Xác thực thành công! Bạn đã nhận được <b>5 Standard credits</b>.\n\n"+
				"API key của bạn sẽ được cấp sau khi hệ thống cập nhật. "+
				"Dùng /key để kiểm tra sau.")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = removeKbd
		_, err = bot.Send(msg)
		return err
	}

	// Show plaintext key ONCE — HTML-escaped, bold warning to save it.
	safeKey := html.EscapeString(plaintext)
	safePrefix := html.EscapeString(prefix)
	text := fmt.Sprintf(
		"Xác thực thành công! Bạn đã nhận được <b>5 Standard credits</b>.\n\n"+
			"<b>API Key của bạn:</b>\n<code>%s</code>\n\n"+
			"Prefix hiển thị: <code>%s...</code>\n\n"+
			"<b>LUU Ý: Key chỉ hiển thị MỘT LẦN. Hãy lưu ngay bây giờ.</b>\n\n"+
			"Dùng /key để xem prefix và /balance để kiểm tra số dư.",
		safeKey, safePrefix,
	)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = removeKbd

	_, err = bot.Send(msg)
	return err
}

// handleTrialError maps domain errors to user-facing messages.
func handleTrialError(bot *tgbotapi.BotAPI, update tgbotapi.Update, trialErr error) error {
	switch {
	case errors.Is(trialErr, service.ErrTrialPhoneReused):
		replyText(bot, update,
			"Số điện thoại này đã được sử dụng để kích hoạt tài khoản khác. "+
				"Mỗi số điện thoại chỉ được dùng một lần.")
		return nil // user-facing error, not a system error

	case errors.Is(trialErr, service.ErrTrialAlreadyUsed):
		replyText(bot, update,
			"Tài khoản của bạn đã được kích hoạt trước đó. "+
				"Dùng /key để xem API key và /balance để kiểm tra số dư.")
		return nil

	case errors.Is(trialErr, service.ErrTrialUserBanned):
		replyText(bot, update,
			"Tài khoản đã bị tạm khoá. Liên hệ @support để được hỗ trợ.")
		return nil

	default:
		replyText(bot, update, "Đã xảy ra lỗi xác thực. Vui lòng thử lại sau.")
		return trialErr // propagate unexpected errors for logging
	}
}
