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

// faqTopics maps callback suffix → canned text.
var faqTopics = map[string]string{
	"payment": "<b>❓ Thanh toán</b>\n\nChúng tôi hỗ trợ chuyển khoản qua SePay. Sau khi chuyển khoản, credits được cộng tự động trong vòng 1 phút. Nếu sau 5 phút chưa nhận được, hãy liên hệ support.",
	"key":     "<b>❓ API Key</b>\n\nKey được cấp một lần duy nhất khi xác thực số điện thoại. Dùng /key để xem prefix. Nếu mất key, dùng /regenkey để tạo key mới (key cũ bị thu hồi).",
	"credits": "<b>❓ Credits</b>\n\nCredits dùng để chạy backlink, captcha, và finder. Premium credits ưu tiên hơn Standard. Xem số dư bằng /balance.",
	"tech":    "<b>❓ Kỹ thuật</b>\n\nNếu bot không phản hồi, thử gửi /start lại. Nếu lỗi liên tục, hãy mô tả chi tiết và gửi ticket để đội kỹ thuật hỗ trợ.",
	"other":   "<b>❓ Khác</b>\n\nVới các vấn đề không có trong FAQ, hãy mô tả chi tiết và gửi ticket. Đội support sẽ phản hồi trong 24h.",
}

// HandleSupport handles the /support command — shows FAQ menu.
func HandleSupport(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	keyboard := faqMenuKeyboard()
	msg := tgbotapi.NewMessage(chatID,
		"🆘 <b>Trung tâm hỗ trợ</b>\n\nChọn chủ đề bạn cần hỗ trợ:")
	msg.ParseMode = tgbotapi.ModeHTML
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
	text, ok := faqTopics[topic]
	if !ok {
		text = "Chủ đề không tìm thấy."
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
		edit.ParseMode = tgbotapi.ModeHTML
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
			"🆘 <b>Trung tâm hỗ trợ</b>\n\nChọn chủ đề bạn cần hỗ trợ:",
		)
		edit.ParseMode = tgbotapi.ModeHTML
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
			"📝 Mô tả vấn đề của bạn (tối đa 4096 ký tự).\n\nGửi /cancel để huỷ.")
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
			"Đã huỷ.",
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
		replyText(api, update,
			"⚠️ Đã đạt giới hạn 3 ticket mở. Đợi admin reply trước khi gửi ticket mới.")
		return nil
	}

	// M5 STEP 4: Insert ticket.
	ticket, err := deps.SupportService.CreateTicket(ctx, user.ID, "Support Ticket", body)
	if err != nil {
		if errors.Is(err, service.ErrSupportTicketCapReached) {
			replyText(api, update,
				"⚠️ Đã đạt giới hạn 3 ticket mở. Đợi admin reply trước khi gửi ticket mới.")
			return nil
		}
		deps.Log.Error("HandleSupportDescribeMessage: CreateTicket failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		replyText(api, update, "⚠️ Không thể gửi ticket. Vui lòng thử lại sau.")
		return nil
	}

	shortID := fmt.Sprintf("%.8s", ticket.ID.String())
	replyText(api, update,
		fmt.Sprintf("✅ Đã gửi ticket #%s. Admin sẽ reply trong vòng 24h.", shortID))
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
