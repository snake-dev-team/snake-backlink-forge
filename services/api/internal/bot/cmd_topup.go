// cmd_topup.go — Phase 05: /topup command + QR resend + topup:cancel callback.
// handleTopupCheckCallback lives in cmd_topup_check.go (file-size split).
//
// /topup:
//   - FSM topup_waiting → resend stored QR photo.
//   - Otherwise → delegate to HandleBuy.
//
// topup:cancel:<tx_id> → CancelPendingTransaction + clear FSM + edit caption.
package bot

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// extractOrderCodeFromQRURL pulls the 12-hex order code from the SePay QR URL's
// `des=SBF TOPUP <CODE>` query parameter. Returns empty string if URL is malformed
// or the description is missing the expected prefix.
func extractOrderCodeFromQRURL(qrURL string) string {
	u, err := url.Parse(qrURL)
	if err != nil {
		return ""
	}
	des := u.Query().Get("des")
	const prefix = "SBF TOPUP "
	if !strings.HasPrefix(des, prefix) {
		return ""
	}
	return strings.TrimPrefix(des, prefix)
}

// HandleTopup handles the /topup command.
// If FSM = topup_waiting, resends the pending QR. Otherwise shows the buy menu.
func HandleTopup(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	if deps.Rdb != nil {
		tgID := updateTelegramID(update)
		st, err := NewStore(deps.Rdb).Load(ctx, tgID)
		if err == nil && st.Name == stateTopupWaiting {
			return resendTopupQR(ctx, deps, api, update, chatID, st)
		}
	}

	return HandleBuy(ctx, deps, api, update)
}

// resendTopupQR extracts QR data from FSM state and resends the QR photo.
// Falls back to HandleBuy if state payload is corrupt or missing QR URL.
func resendTopupQR(
	ctx context.Context,
	deps *Deps,
	api *tgbotapi.BotAPI,
	update tgbotapi.Update,
	chatID int64,
	st State,
) error {
	var data topupWaitingData
	if err := json.Unmarshal(st.Data, &data); err != nil || data.QRUrl == "" {
		_ = NewStore(deps.Rdb).Clear(ctx, updateTelegramID(update))
		return HandleBuy(ctx, deps, api, update)
	}

	txID, err := uuid.Parse(data.TxID)
	if err != nil {
		_ = NewStore(deps.Rdb).Clear(ctx, updateTelegramID(update))
		return HandleBuy(ctx, deps, api, update)
	}

	pkg := service.Packages[data.PackageCode]
	// [H1 fix] Use the actual QR caption template (KeyTopupQRCaption) — not buy_confirm
	// which prompts "Xác nhận thanh toán?" inappropriate after user has already confirmed.
	// Order code parsed from the QR URL's `des` query param to avoid FSM schema change.
	bankCode := ""
	if deps.Cfg != nil {
		bankCode = deps.Cfg.SepayBankCode
	}
	caption := renderTplCtx(ctx, deps, tplTopupQRCaption, struct {
		Display            string
		BankCode           string
		AmountVNDFormatted string
		OrderCode          string
	}{
		Display:            pkg.DisplayVI,
		BankCode:           bankCode,
		AmountVNDFormatted: formatVND(pkg.AmountVND),
		OrderCode:          extractOrderCodeFromQRURL(data.QRUrl),
	})

	keyboard := TopupActionsKeyboard(txID)
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(data.QRUrl))
	photo.Caption = caption
	photo.ParseMode = "Markdown"
	photo.ReplyMarkup = keyboard

	if _, sendErr := api.Send(photo); sendErr != nil {
		deps.Log.Warn("resendTopupQR: send photo failed, text fallback", zap.Error(sendErr))
		msg := tgbotapi.NewMessage(chatID, caption+"\n\n🔗 "+data.QRUrl)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		_, _ = api.Send(msg)
	}
	return nil
}

// handleTopupCancelCallback handles "topup:cancel:<tx_id>":
// cancels the pending transaction ([Q2] → status='cancelled'), clears FSM, edits caption.
func handleTopupCancelCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	txIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "topup:cancel:")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		replyText(api, update, "⚠️ ID giao dịch không hợp lệ.")
		return nil
	}

	// [H1] Ownership check — reject cancel from non-owner. Fetch tx + compare user_id.
	// Attacker with leaked txID can't cancel victim's pending tx.
	botUser, userOK := UserFromCtx(ctx)
	if !userOK {
		deps.Log.Warn("handleTopupCancelCallback: no user in ctx", zap.String("tx_id", txIDStr))
		replyText(api, update, "⚠️ Không tìm thấy giao dịch.")
		return nil
	}
	tx, fetchErr := fetchTxByID(ctx, deps, txID)
	if fetchErr != nil || tx.UserID != botUser.ID {
		deps.Log.Warn("handleTopupCancelCallback: tx ownership mismatch or missing",
			zap.String("tx_id", txIDStr), zap.Bool("fetch_ok", fetchErr == nil))
		replyText(api, update, "⚠️ Không tìm thấy giao dịch.")
		return nil
	}

	if deps.TxService != nil {
		if cancelErr := deps.TxService.CancelPendingTransaction(ctx, txID, "user_clicked_cancel"); cancelErr != nil {
			deps.Log.Warn("handleTopupCancelCallback: cancel failed",
				zap.String("tx_id", txIDStr), zap.Error(cancelErr))
			// Non-fatal: still clear FSM and update UI.
		}
	}

	if deps.Rdb != nil {
		_ = NewStore(deps.Rdb).Clear(ctx, updateTelegramID(update))
	}

	cancelMsg := renderTplCtx(ctx, deps, tplTopupCancelled, nil)
	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageCaption(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			cancelMsg,
		)
		if _, editErr := api.Send(edit); editErr != nil {
			deps.Log.Debug("handleTopupCancelCallback: edit caption failed", zap.Error(editErr))
			editText := tgbotapi.NewEditMessageText(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				cancelMsg,
			)
			_, _ = api.Send(editText)
		}
	}
	return nil
}
