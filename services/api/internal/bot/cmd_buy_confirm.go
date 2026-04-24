// cmd_buy_confirm.go — Phase 05: buy:confirm + buy:cancel callbacks.
// Separated from cmd_buy.go to keep files under 200 lines.
// handleBuyConfirmCallback: creates topup intent → sends QR photo → FSM topup_waiting.
// handleBuyCancelCallback:  clears FSM → edits message to "Đã huỷ."
package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// handleBuyConfirmCallback handles "buy:confirm:<code>":
//  1. Validates package code (defense-in-depth).
//  2. Calls TransactionService.CreateTopupIntent (idempotent via idx_tx_user_pkg_pending).
//  3. Sends QR photo with caption + TopupActionsKeyboard.
//  4. Transitions FSM → topup_waiting (TTL 24h).
func handleBuyConfirmCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	pkgCode := strings.TrimPrefix(update.CallbackQuery.Data, "buy:confirm:")
	if err := service.ValidatePackageCode(pkgCode); err != nil {
		deps.Log.Warn("handleBuyConfirmCallback: invalid pkg code", zap.String("data", update.CallbackQuery.Data))
		replyText(api, update, "⚠️ Gói không hợp lệ.")
		return nil
	}

	user, ok := UserFromCtx(ctx)
	if !ok || deps.TxService == nil {
		replyText(api, update, "⚠️ Hệ thống gặp sự cố. Vui lòng thử lại sau.")
		return nil
	}

	chatID := updateChatID(update)

	tx, qrURL, err := deps.TxService.CreateTopupIntent(ctx, user.ID, pkgCode)
	if err != nil {
		deps.Log.Error("handleBuyConfirmCallback: CreateTopupIntent failed",
			zap.String("user_id", user.ID.String()),
			zap.String("pkg", pkgCode),
			zap.Error(err),
		)
		msg := tgbotapi.NewMessage(chatID, "⚠️ Không thể tạo lệnh thanh toán. Vui lòng thử lại.")
		_, _ = api.Send(msg)
		return err
	}

	pkg := service.Packages[pkgCode]
	ref := ""
	if tx.ProviderRef != nil {
		ref = *tx.ProviderRef
	}

	caption := fmt.Sprintf(
		"💳 *Thanh toán gói %s*\n\n"+
			"🏦 Ngân hàng: %s\n"+
			"💰 Số tiền: *%sđ*\n"+
			"📝 Nội dung CK: <code>SBF TOPUP %s</code>\n\n"+
			"⏰ QR có hiệu lực 24 giờ.",
		pkg.DisplayVI,
		deps.Cfg.SepayBankCode,
		formatVND(pkg.AmountVND),
		ref,
	)

	keyboard := TopupActionsKeyboard(tx.ID)
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(qrURL))
	photo.Caption = caption
	photo.ParseMode = tgbotapi.ModeHTML
	photo.ReplyMarkup = keyboard

	if _, sendErr := api.Send(photo); sendErr != nil {
		deps.Log.Error("handleBuyConfirmCallback: send QR photo failed",
			zap.String("qr_url", qrURL), zap.Error(sendErr))
		fallback := tgbotapi.NewMessage(chatID, caption+"\n\n🔗 "+qrURL)
		fallback.ParseMode = tgbotapi.ModeHTML
		fallback.ReplyMarkup = keyboard
		_, _ = api.Send(fallback)
	}

	// Transition FSM → topup_waiting.
	if deps.Rdb != nil {
		tgID := updateTelegramID(update)
		payload, _ := json.Marshal(topupWaitingData{
			TxID:        tx.ID.String(),
			PackageCode: pkgCode,
			QRUrl:       qrURL,
			ExpiresAt:   time.Now().Add(topupWaitingTTL).Unix(),
		})
		_ = NewStore(deps.Rdb).Save(ctx, tgID, State{Name: stateTopupWaiting, Data: payload}, topupWaitingTTL)
	}
	return nil
}

// handleBuyCancelCallback handles "buy:cancel" — clears FSM and edits the message to "Đã huỷ."
func handleBuyCancelCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	if deps.Rdb != nil {
		_ = NewStore(deps.Rdb).Clear(ctx, updateTelegramID(update))
	}

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			"Đã huỷ.",
		)
		if _, err := api.Send(edit); err != nil {
			deps.Log.Debug("handleBuyCancelCallback: edit failed", zap.Error(err))
		}
	}
	return nil
}
