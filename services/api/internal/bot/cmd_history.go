// cmd_history.go — Phase 07: /history command with paginated tx + ledger consume view.
// Stateless pagination: page index encoded in callback data (no FSM needed).
// Two independent pages: hist:tx:<page> and hist:ledger:<page> (0-indexed).
package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

const historyPageSize = 5

// HandleHistory handles the /history command. Shows page 0 of both sections.
func HandleHistory(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	user, ok := UserFromCtx(ctx)
	if !ok || user.ID.String() == "00000000-0000-0000-0000-000000000000" {
		replyText(api, update, "⚠️ Vui lòng /start để khởi tạo tài khoản.")
		return nil
	}

	text, keyboard, err := buildHistoryMessage(ctx, deps, user, 0, 0)
	if err != nil {
		deps.Log.Error("HandleHistory: buildHistoryMessage failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		replyText(api, update, "⚠️ Không thể tải lịch sử. Vui lòng thử lại.")
		return nil
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if keyboard != nil {
		msg.ReplyMarkup = keyboard
	}
	_, err = api.Send(msg)
	return err
}

// HandleHistoryTxPageCallback handles "hist:tx:<page>" callbacks.
func HandleHistoryTxPageCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	return handleHistoryCallback(ctx, deps, api, update, true)
}

// HandleHistoryLedgerPageCallback handles "hist:ledger:<page>" callbacks.
func HandleHistoryLedgerPageCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	return handleHistoryCallback(ctx, deps, api, update, false)
}

// handleHistoryCallback is the shared callback handler for both pagination axes.
// isTxPage=true → update tx page; isTxPage=false → update ledger page.
func handleHistoryCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update, isTxPage bool) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	user, ok := UserFromCtx(ctx)
	if !ok {
		return nil
	}

	data := update.CallbackQuery.Data
	var pageTx, pageLedger int

	// Parse the page number from callback data; also recover the other page from message text is not feasible,
	// so we default both to 0 and only set the one being navigated.
	// The other axis resets to 0 on cross-axis navigation (acceptable UX per spec).
	if isTxPage {
		fmt.Sscanf(strings.TrimPrefix(data, "hist:tx:"), "%d", &pageTx)
	} else {
		fmt.Sscanf(strings.TrimPrefix(data, "hist:ledger:"), "%d", &pageLedger)
	}

	text, keyboard, err := buildHistoryMessage(ctx, deps, user, pageTx, pageLedger)
	if err != nil {
		deps.Log.Error("handleHistoryCallback: buildHistoryMessage failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		return nil
	}

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			text,
		)
		edit.ParseMode = tgbotapi.ModeHTML
		if keyboard != nil {
			edit.ReplyMarkup = keyboard
		}
		_, _ = api.Send(edit)
	}
	return nil
}

// buildHistoryMessage fetches both data sources and renders the combined message + pagination keyboard.
func buildHistoryMessage(ctx context.Context, deps *Deps, user BotUser, pageTx, pageLedger int) (string, *tgbotapi.InlineKeyboardMarkup, error) {
	q := sqlcdb.New(deps.Pool)

	txs, totalTx, err := fetchTxPage(ctx, q, user, pageTx)
	if err != nil {
		return "", nil, err
	}

	consumes, totalLedger, err := fetchLedgerConsumePage(ctx, q, user, pageLedger)
	if err != nil {
		return "", nil, err
	}

	if len(txs) == 0 && len(consumes) == 0 {
		return renderTplCtx(ctx, deps, tplHistoryEmpty, nil), nil, nil
	}

	var sb strings.Builder
	sb.WriteString("<b>📋 Lịch sử giao dịch</b>\n")
	if len(txs) == 0 {
		sb.WriteString("  (chưa có giao dịch)\n")
	} else {
		for _, tx := range txs {
			sb.WriteString(formatTxRow(tx))
		}
	}

	sb.WriteString("\n<b>⚡ Lịch sử tiêu credits</b>\n")
	if len(consumes) == 0 {
		sb.WriteString("  (chưa có)\n")
	} else {
		for _, l := range consumes {
			sb.WriteString(formatLedgerRow(l))
		}
	}

	keyboard := buildHistoryKeyboard(pageTx, pageLedger, int(totalTx), int(totalLedger))
	return sb.String(), keyboard, nil
}

// fetchTxPage returns paginated transactions + total count.
func fetchTxPage(ctx context.Context, q *sqlcdb.Queries, user BotUser, page int) ([]sqlcdb.Transaction, int64, error) {
	total, err := q.CountTxByUser(ctx, user.ID)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	offset := int32(page * historyPageSize)
	txs, err := q.GetTxByUserPage(ctx, sqlcdb.GetTxByUserPageParams{
		UserID: user.ID,
		Limit:  historyPageSize,
		Offset: offset,
	})
	return txs, total, err
}

// fetchLedgerConsumePage returns paginated consume ledger rows + total count.
func fetchLedgerConsumePage(ctx context.Context, q *sqlcdb.Queries, user BotUser, page int) ([]sqlcdb.Ledger, int64, error) {
	total, err := q.CountLedgerConsumesByUser(ctx, user.ID)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	offset := int32(page * historyPageSize)
	rows, err := q.GetLedgerConsumesByUserPage(ctx, sqlcdb.GetLedgerConsumesByUserPageParams{
		UserID: user.ID,
		Limit:  historyPageSize,
		Offset: offset,
	})
	return rows, total, err
}

// txStatusIcon maps transaction status to a display icon.
func txStatusIcon(status sqlcdb.TransactionStatus) string {
	switch status {
	case sqlcdb.TransactionStatusPending:
		return "⏳"
	case sqlcdb.TransactionStatusPaid:
		return "✅"
	case sqlcdb.TransactionStatusCancelled:
		return "❌"
	case sqlcdb.TransactionStatusManualReview:
		return "⚠️"
	case sqlcdb.TransactionStatusRecoveredByLatePayment:
		return "🔄"
	case sqlcdb.TransactionStatusFailed:
		return "💔"
	default:
		return "❓"
	}
}

// formatTxRow renders a single transaction as an HTML line.
func formatTxRow(tx sqlcdb.Transaction) string {
	icon := txStatusIcon(tx.Status)
	date := tx.CreatedAt.Format("02/01 15:04")
	return fmt.Sprintf("%s %s · %sđ · %s\n",
		icon, tx.PackageCode, formatVND(tx.AmountVnd), date)
}

// ledgerEventLabel returns a short Vietnamese label for a ledger event type.
func ledgerEventLabel(et sqlcdb.LedgerEventType) string {
	switch et {
	case sqlcdb.LedgerEventTypeConsumeBacklink:
		return "Backlink"
	case sqlcdb.LedgerEventTypeConsumeCaptcha:
		return "Captcha"
	case sqlcdb.LedgerEventTypeConsumeFinder:
		return "Finder"
	default:
		return string(et)
	}
}

// formatLedgerRow renders a single ledger consume row as an HTML line.
func formatLedgerRow(l sqlcdb.Ledger) string {
	label := ledgerEventLabel(l.EventType)
	date := l.CreatedAt.Format("02/01 15:04")
	delta := l.DeltaCredits // negative for consume
	return fmt.Sprintf("▪ %s · %d · %s\n", label, delta, date)
}

// buildHistoryKeyboard returns pagination buttons for both axes, or nil if none needed.
func buildHistoryKeyboard(pageTx, pageLedger, totalTx, totalLedger int) *tgbotapi.InlineKeyboardMarkup {
	maxPageTx := maxPage(totalTx)
	maxPageLedger := maxPage(totalLedger)

	hasTxNav := pageTx > 0 || pageTx < maxPageTx
	hasLedgerNav := pageLedger > 0 || pageLedger < maxPageLedger

	if !hasTxNav && !hasLedgerNav {
		return nil
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	// Tx pagination row.
	if hasTxNav {
		row := buildPaginationRow("hist:tx", pageTx, maxPageTx, "Tx")
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	// Ledger pagination row.
	if hasLedgerNav {
		row := buildPaginationRow("hist:ledger", pageLedger, maxPageLedger, "Credits")
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	if len(rows) == 0 {
		return nil
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}

// maxPage returns the last valid page index (0-indexed) for a given total item count.
func maxPage(total int) int {
	if total <= 0 {
		return 0
	}
	p := (total - 1) / historyPageSize
	if p < 0 {
		return 0
	}
	return p
}

// buildPaginationRow builds ◀ prev / next ▶ buttons for one axis.
// prefix is the callback prefix (e.g. "hist:tx"), label is the section label.
func buildPaginationRow(prefix string, page, maxPageIdx int, label string) []tgbotapi.InlineKeyboardButton {
	var btns []tgbotapi.InlineKeyboardButton
	if page > 0 {
		btns = append(btns, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("◀ %s", label),
			fmt.Sprintf("%s:%d", prefix, page-1),
		))
	}
	if page < maxPageIdx {
		btns = append(btns, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s ▶", label),
			fmt.Sprintf("%s:%d", prefix, page+1),
		))
	}
	return btns
}

