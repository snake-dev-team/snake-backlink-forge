// cmd_support.go — Phase 07: /support command + FAQ menu + ticket submission FSM.
//
// FSM states:
//   support_describing — user was prompted to describe their issue (TTL 30min)
//
// M5 ordering (critical): on state=support_describing text message:
//   1. Clear state FIRST (one-shot guarantee — no re-entry even if reply fails)
//   2. Truncate body to 4096 bytes
//   3. Check cap (>= 3 open tickets → reject)
//   4. Insert ticket → reply confirmation
package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

const (
	stateSupportDescribing = "support_describing"
	supportDescribingTTL   = 30 * time.Minute
)

// faqTopicKeys maps the callback suffix to the template key.
// "other" reuses tech FAQ — no dedicated bundle entry needed (kept inline as
// final fallback to avoid bundle bloat for low-impact copy).
var faqTopicKeys = map[string]string{
	"payment": tplSupportFAQPayment,
	"key":     tplSupportFAQKey,
	"credits": tplSupportFAQCredits,
	"tech":    tplSupportFAQTechnical,
	"other":   tplSupportFAQTechnical,
}

// HandleSupport handles the /support command — shows FAQ menu.
func HandleSupport(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	keyboard := faqMenuKeyboard()
	msg := tgbotapi.NewMessage(chatID, renderTplCtx(ctx, deps, tplSupportMenu, nil))
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	_, err := api.Send(msg)
	return err
}

// HandleSupportFAQCallback handles "support:faq:<topic>" — edits message with canned text.
func HandleSupportFAQCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	topic := strings.TrimPrefix(update.CallbackQuery.Data, "support:faq:")
	tplKey, ok := faqTopicKeys[topic]
	var text string
	if !ok {
		text = renderTplCtx(ctx, deps, tplErrorGeneric, nil)
	} else {
		text = renderTplCtx(ctx, deps, tplKey, nil)
	}

	// Add "Vẫn cần hỗ trợ" + Cancel buttons below FAQ text.
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💬 Vẫn cần hỗ trợ? Bấm đây", "support:describe"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Quay lại FAQ", "support:menu"),
		),
	)

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			text,
		)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		_, _ = api.Send(edit)
	}
	return nil
}

// HandleSupportMenuCallback handles "support:menu" — restores FAQ menu view.
func HandleSupportMenuCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	keyboard := faqMenuKeyboard()
	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			renderTplCtx(ctx, deps, tplSupportMenu, nil),
		)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		_, _ = api.Send(edit)
	}
	return nil
}

// HandleSupportDescribeCallback handles "support:describe" — sets FSM state + prompts user.
func HandleSupportDescribeCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	tgID := updateTelegramID(update)
	if deps.Rdb != nil {
		_ = NewStore(deps.Rdb).Save(ctx, tgID, State{Name: stateSupportDescribing}, supportDescribingTTL)
	}

	chatID := updateChatID(update)
	if chatID != 0 {
		msg := tgbotapi.NewMessage(chatID,
			renderTplCtx(ctx, deps, tplSupportDescribePrompt, nil))
		_, _ = api.Send(msg)
	}
	return nil
}

// HandleSupportCancelCallback handles "support:cancel" callback — clears FSM.
func HandleSupportCancelCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	tgID := updateTelegramID(update)
	if deps.Rdb != nil {
		_ = NewStore(deps.Rdb).Clear(ctx, tgID)
	}

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			renderTplCtx(ctx, deps, tplBuyCancelled, nil),
		)
		_, _ = api.Send(edit)
	}
	return nil
}

// HandleSupportDescribeMessage handles a plain text message when FSM = support_describing.
// M5 ordering: Clear state FIRST, then truncate, cap-check, insert.
func HandleSupportDescribeMessage(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}

	tgID := updateTelegramID(update)

	// M5 STEP 1: Clear state FIRST — one-shot guarantee.
	// Even if everything below fails, state is gone; next message hits default handler.
	if deps.Rdb != nil {
		_ = NewStore(deps.Rdb).Clear(ctx, tgID)
	}

	// M5 STEP 2: Truncate body to 4096 bytes (byte boundary per spec).
	body := update.Message.Text
	if len(body) > service.SupportTicketBodyMaxBytes {
		body = body[:service.SupportTicketBodyMaxBytes]
	}

	user, ok := UserFromCtx(ctx)
	if !ok {
		replyText(api, update, "⚠️ Không tìm thấy tài khoản. Vui lòng /start.")
		return nil
	}

	if deps.SupportService == nil {
		replyText(api, update, "⚠️ Hệ thống hỗ trợ chưa sẵn sàng. Vui lòng thử lại sau.")
		return nil
	}

	// M5 STEP 3: Check cap.
	count, err := deps.SupportService.CountOpenTickets(ctx, user.ID)
	if err != nil {
		deps.Log.Error("HandleSupportDescribeMessage: CountOpenTickets failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		replyText(api, update, "⚠️ Hệ thống gặp sự cố. Vui lòng thử lại sau.")
		return nil
	}
	if count >= service.SupportTicketCap {
		replyText(api, update, renderTplCtx(ctx, deps, tplSupportTicketCapReached, nil))
		return nil
	}

	// M5 STEP 4: Insert ticket.
	ticket, err := deps.SupportService.CreateTicket(ctx, user.ID, "Support Ticket", body)
	if err != nil {
		if errors.Is(err, service.ErrSupportTicketCapReached) {
			replyText(api, update, renderTplCtx(ctx, deps, tplSupportTicketCapReached, nil))
			return nil
		}
		deps.Log.Error("HandleSupportDescribeMessage: CreateTicket failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		replyText(api, update, renderTplCtx(ctx, deps, tplErrorGeneric, nil))
		return nil
	}

	shortID := fmt.Sprintf("%.8s", ticket.ID.String())
	replyText(api, update, renderTplCtx(ctx, deps, tplSupportTicketSubmitted, struct {
		TicketID string
	}{
		TicketID: shortID,
	}))
	return nil
}

// faqMenuKeyboard returns the 5-button FAQ topic keyboard.
func faqMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Thanh toán", "support:faq:payment"),
			tgbotapi.NewInlineKeyboardButtonData("🔑 Key", "support:faq:key"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Credits", "support:faq:credits"),
			tgbotapi.NewInlineKeyboardButtonData("🛠 Kỹ thuật", "support:faq:tech"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❓ Khác", "support:faq:other"),
		),
	)
}
