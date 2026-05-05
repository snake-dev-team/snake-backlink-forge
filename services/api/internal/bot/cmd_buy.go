// cmd_buy.go — Phase 05: /buy command + package-selection callback.
//
// FSM states managed here:
//   buy_selecting_package — user sees package menu
//   buy_confirming        — user sees package summary + Confirm/Cancel
//
// handleBuyConfirmCallback + handleBuyCancelCallback live in cmd_buy_confirm.go.
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

const (
	stateBuySelectingPackage = "buy_selecting_package"
	stateBuyConfirming       = "buy_confirming"
	stateTopupWaiting        = "topup_waiting"

	buyConfirmTTL   = 5 * time.Minute
	topupWaitingTTL = 24 * time.Hour
)

// topupWaitingData is the FSM payload stored in state.Data for stateTopupWaiting.
type topupWaitingData struct {
	TxID        string `json:"tx_id"`
	PackageCode string `json:"package_code"`
	QRUrl       string `json:"qr_url"`
	ExpiresAt   int64  `json:"expires_at"`
}

// buyConfirmingData is the FSM payload stored in state.Data for stateBuyConfirming.
type buyConfirmingData struct {
	PackageCode string `json:"package_code"`
}

// HandleBuy handles the /buy command. Sends the package selection keyboard
// and sets FSM state to buy_selecting_package.
func HandleBuy(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	if deps.TxService == nil {
		msg := tgbotapi.NewMessage(chatID, "Tính năng mua gói credit sắp ra mắt.")
		_, err := api.Send(msg)
		return err
	}

	keyboard := PackageMenuKeyboard(LangFromCtx(ctx))
	msg := tgbotapi.NewMessage(chatID, renderTplCtx(ctx, deps, tplBuyMenuHeader, nil))
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	if deps.Rdb != nil {
		tgID := updateTelegramID(update)
		_ = NewStore(deps.Rdb).Save(ctx, tgID, State{Name: stateBuySelectingPackage}, buyConfirmTTL)
	}

	_, err := api.Send(msg)
	return err
}

// handleBuyPackageCallback handles "buy:pkg:<code>" — validates package,
// shows summary with Confirm/Cancel, transitions FSM to buy_confirming.
func handleBuyPackageCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	pkgCode := strings.TrimPrefix(update.CallbackQuery.Data, "buy:pkg:")
	if err := service.ValidatePackageCode(pkgCode); err != nil {
		deps.Log.Warn("handleBuyPackageCallback: invalid pkg code", zap.String("data", update.CallbackQuery.Data))
		replyText(api, update, "⚠️ Gói không hợp lệ.")
		return nil
	}

	pkg := service.Packages[pkgCode]
	text := renderTplCtx(ctx, deps, tplBuyConfirm, struct {
		Display            string
		CreditSummary      string
		AmountVNDFormatted string
	}{
		Display:            pkg.DisplayVI,
		CreditSummary:      buildCreditSummary(pkg),
		AmountVNDFormatted: formatVND(pkg.AmountVND),
	})
	keyboard := ConfirmCancelKeyboard(pkgCode)

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID, text,
		)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		if _, err := api.Send(edit); err != nil {
			deps.Log.Debug("handleBuyPackageCallback: edit failed, sending new", zap.Error(err))
			msg := tgbotapi.NewMessage(updateChatID(update), text)
			msg.ParseMode = "Markdown"
			msg.ReplyMarkup = keyboard
			_, _ = api.Send(msg)
		}
	}

	if deps.Rdb != nil {
		tgID := updateTelegramID(update)
		data, _ := json.Marshal(buyConfirmingData{PackageCode: pkgCode})
		_ = NewStore(deps.Rdb).Save(ctx, tgID, State{Name: stateBuyConfirming, Data: data}, buyConfirmTTL)
	}
	return nil
}

// buildCreditSummary returns a human-readable credits description for a package.
func buildCreditSummary(pkg service.Package) string {
	switch {
	case pkg.PremiumCredits > 0 && pkg.StandardCredits > 0:
		return fmt.Sprintf("%d Premium + %d Standard", pkg.PremiumCredits, pkg.StandardCredits)
	case pkg.PremiumCredits > 0:
		return fmt.Sprintf("%d Premium", pkg.PremiumCredits)
	default:
		return fmt.Sprintf("%d Standard", pkg.StandardCredits)
	}
}
