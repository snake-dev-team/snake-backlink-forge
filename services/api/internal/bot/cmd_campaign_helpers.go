// cmd_campaign_helpers.go — shared helpers for /campaign subcommands.
// Covers: ID prefix resolution, Markdown V1 escaping, error formatting,
// and pagination keyboard builder.
package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
)

// resolveCampaignID resolves a user-supplied ID prefix to a full UUID.
// Returns ErrNotFound (as plain error string) when 0 rows match.
// Returns ambiguity error when 2 rows match (LIMIT 2 in SQL).
// prefix must be at least 4 chars; shorter inputs are rejected eagerly.
func resolveCampaignID(ctx context.Context, deps *Deps, userID uuid.UUID, prefix string) (uuid.UUID, error) {
	if len(prefix) < 4 {
		return uuid.Nil, errors.New("ID quá ngắn (min 4 ký tự)")
	}
	if deps.CampaignService == nil {
		return uuid.Nil, errors.New("⚠️ Tính năng campaign chưa sẵn sàng.")
	}
	rows, err := deps.CampaignService.ResolveIDPrefix(ctx, userID, prefix)
	if err != nil {
		return uuid.Nil, errors.New("⚠️ Lỗi tra cứu campaign.")
	}
	if len(rows) == 0 {
		return uuid.Nil, errors.New("Không tìm thấy campaign khớp.")
	}
	if len(rows) > 1 {
		return uuid.Nil, errors.New("ID prefix không duy nhất, gõ thêm ký tự.")
	}
	return rows[0].ID, nil
}

// formatCampaignErr converts known service errors to Vietnamese user messages.
func formatCampaignErr(err error) string {
	switch {
	case errors.Is(err, service.ErrCampaignInvalid):
		return "Dữ liệu không hợp lệ (URL/credit/limit)."
	case errors.Is(err, service.ErrCampaignNotFound):
		return "Không tìm thấy campaign."
	case errors.Is(err, service.ErrCampaignUnavailable):
		return "⚠️ Tính năng campaign chưa sẵn sàng."
	default:
		return "⚠️ Lỗi hệ thống. Thử lại sau."
	}
}

// escapeMD escapes Telegram Markdown V1 special characters in user-supplied strings.
// Must be applied to: campaign name, money_url hostname, anchor text, error messages.
// NOT needed for: status enums, integer counts.
func escapeMD(s string) string {
	repl := strings.NewReplacer(
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
	)
	return repl.Replace(s)
}

// buildPagerKeyboard returns an inline keyboard with prev/next navigation.
// prefix is the callback data prefix (e.g. "cmp:list", "cmp:jobs:<id>").
// Only renders "Sau" when hasNext=true, only renders "Trước" when page>0.
func buildPagerKeyboard(prefix string, page int, hasNext bool) tgbotapi.InlineKeyboardMarkup {
	var btns []tgbotapi.InlineKeyboardButton
	if page > 0 {
		btns = append(btns, tgbotapi.NewInlineKeyboardButtonData(
			"◀️ Trước",
			fmt.Sprintf("%s:%d", prefix, page-1),
		))
	}
	if hasNext {
		btns = append(btns, tgbotapi.NewInlineKeyboardButtonData(
			"Sau ▶️",
			fmt.Sprintf("%s:%d", prefix, page+1),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btns...))
}
