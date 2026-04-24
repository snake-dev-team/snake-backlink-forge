// cmd_topup_check.go — Phase 05: topup:check:<tx_id> callback handler.
// Polls DB for payment status once per user tap.
// Paid/recovered → success message + clear FSM.
// Still pending → AnswerCallbackQuery alert popup (no new chat message).
package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// handleTopupCheckCallback handles "topup:check:<tx_id>":
//   - If status = paid or recovered_by_late_payment → success message + clear FSM.
//   - Otherwise → AnswerCallbackQuery alert "still waiting" (no new message in chat).
func handleTopupCheckCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}

	txIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "topup:check:")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "ID không hợp lệ.")
		_, _ = api.Request(cb)
		return nil
	}

	if deps.TxService == nil {
		cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "⚠️ Hệ thống gặp sự cố.")
		_, _ = api.Request(cb)
		return nil
	}

	// Try by UUID directly (most reliable path).
	tx, fetchErr := fetchTxByID(ctx, deps, txID)
	if fetchErr != nil {
		deps.Log.Warn("handleTopupCheckCallback: fetchTxByID failed",
			zap.String("tx_id", txIDStr), zap.Error(fetchErr))
		cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "Không tìm thấy giao dịch.")
		_, _ = api.Request(cb)
		return nil
	}

	// [H1] Ownership check — reject if callback txID belongs to another user.
	// Attacker with leaked txID can't query victim's package info or trigger state changes.
	botUser, ok := UserFromCtx(ctx)
	if !ok || tx.UserID != botUser.ID {
		deps.Log.Warn("handleTopupCheckCallback: tx ownership mismatch",
			zap.String("tx_id", txIDStr), zap.Bool("user_ctx_ok", ok))
		cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "Không tìm thấy giao dịch.")
		_, _ = api.Request(cb)
		return nil
	}

	if tx.Status == sqlcdb.TransactionStatusPaid ||
		tx.Status == sqlcdb.TransactionStatusRecoveredByLatePayment {

		// Payment confirmed — clear FSM and send success message.
		if deps.Rdb != nil {
			_ = NewStore(deps.Rdb).Clear(ctx, updateTelegramID(update))
		}

		cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "✅ Thanh toán thành công!")
		_, _ = api.Request(cb)

		pkg := service.Packages[tx.PackageCode]
		msg := tgbotapi.NewMessage(updateChatID(update),
			fmt.Sprintf(
				"✅ *Nạp tiền thành công!*\n\n📦 Gói: %s\n💳 Credits: %s\n\nDùng /balance để xem số dư.",
				pkg.DisplayVI, buildCreditSummary(pkg),
			),
		)
		msg.ParseMode = "Markdown"
		_, _ = api.Send(msg)
		return nil
	}

	// Payment not yet received — show popup alert, no new chat message.
	cb := tgbotapi.NewCallbackWithAlert(
		update.CallbackQuery.ID,
		"⏳ Đang chờ... chưa nhận được thanh toán. Vui lòng đợi tối đa 5 phút sau khi chuyển khoản.",
	)
	cb.ShowAlert = true
	_, _ = api.Request(cb)
	return nil
}

// fetchTxByID queries a transaction directly by its UUID primary key.
// Fallback used when provider_ref lookup is not available.
func fetchTxByID(ctx context.Context, deps *Deps, txID uuid.UUID) (sqlcdb.Transaction, error) {
	if deps.Pool == nil {
		return sqlcdb.Transaction{}, errors.New("no db pool")
	}
	var tx sqlcdb.Transaction
	err := deps.Pool.QueryRow(ctx,
		`SELECT id, user_id, provider, provider_ref, package_code, amount_vnd,
		        premium_granted, standard_granted, status, paid_at, metadata, created_at, updated_at
		 FROM transactions WHERE id = $1`, txID,
	).Scan(
		&tx.ID, &tx.UserID, &tx.Provider, &tx.ProviderRef, &tx.PackageCode,
		&tx.AmountVnd, &tx.PremiumGranted, &tx.StandardGranted, &tx.Status,
		&tx.PaidAt, &tx.Metadata, &tx.CreatedAt, &tx.UpdatedAt,
	)
	if err != nil {
		return sqlcdb.Transaction{}, fmt.Errorf("fetchTxByID: %w", err)
	}
	return tx, nil
}
