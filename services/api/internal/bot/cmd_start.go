// Package bot — /start command + contact-share handler.
// Phase 02: implements user upsert, FSM awaiting_contact, phone verify, trial grant.
package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/bot/templates"
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
			renderTplCtx(ctx, deps, tplStartVerifiedRepeat, nil))
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
		renderTplCtx(ctx, deps, templates.KeyStartWelcome, nil))
	msg.ParseMode = "Markdown"
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

	if contact.UserID == 0 || contact.UserID != tgID {
		replyText(bot, update, renderTplCtx(ctx, deps, tplStartContactRejected, nil))
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
		return handleTrialError(ctx, deps, bot, update, trialErr)
	}

	// Remove the contact keyboard (send RemoveKeyboard).
	removeKbd := tgbotapi.NewRemoveKeyboard(true)

	if plaintext == "" {
		// noopKeyIssuer — key not yet wired (Phase 03 pending). Render the
		// templated success message but with empty Key field; the template
		// still renders cleanly (backtick-wrapped empty string is harmless).
		text := renderTplCtx(ctx, deps, templates.KeyStartVerifiedFirst, struct {
			Key             string
			StandardCredits int
		}{
			Key:             "",
			StandardCredits: 5,
		})
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = removeKbd
		_, err = bot.Send(msg)
		return err
	}

	// Show plaintext key ONCE. Template wraps {{.Key}} in backticks (Markdown
	// code-span). API keys are alphanumeric (sbf_live_<base58>), no escape needed.
	// Prefix is omitted from the templated body — users can run /key after.
	text := renderTplCtx(ctx, deps, templates.KeyStartVerifiedFirst, struct {
		Key             string
		StandardCredits int
	}{
		Key:             plaintext,
		StandardCredits: 5,
	})
	_ = prefix // prefix logged elsewhere; not displayed in v1 templated message
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = removeKbd

	_, err = bot.Send(msg)
	return err
}

// handleTrialError maps domain errors to user-facing messages.
// Error semantics are preserved exactly; only the rendered text uses templates.
func handleTrialError(ctx context.Context, deps *Deps, bot *tgbotapi.BotAPI, update tgbotapi.Update, trialErr error) error {
	lang := LangFromCtx(ctx)
	switch {
	case errors.Is(trialErr, service.ErrTrialPhoneReused):
		replyText(bot, update, renderTpl(deps, lang, tplTrialPhoneReused, nil))
		return nil // user-facing error, not a system error

	case errors.Is(trialErr, service.ErrTrialAlreadyUsed):
		replyText(bot, update, renderTpl(deps, lang, tplStartVerifiedRepeat, nil))
		return nil

	case errors.Is(trialErr, service.ErrTrialUserBanned):
		replyText(bot, update, renderTpl(deps, lang, tplStartTrialBlockedBan, nil))
		return nil

	default:
		replyText(bot, update, renderTpl(deps, lang, tplErrorGeneric, nil))
		return trialErr // propagate unexpected errors for logging
	}
}
