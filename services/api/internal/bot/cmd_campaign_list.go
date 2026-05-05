// cmd_campaign_list.go — Phase 7.02: /campaign list + pagination callback.
// Shows latest campaigns for the user, 10 per page, with inline pager.
package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

const campaignListPageSize = 10

// HandleCampaignList handles /campaign list and cmp:list:<page> callbacks.
// page is 0-indexed.
func HandleCampaignList(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update, page int) error {
	user, ok := UserFromCtx(ctx)
	if !ok {
		replyText(api, update, "⚠️ Vui lòng /start trước.")
		return nil
	}

	limit := int32(campaignListPageSize + 1) // fetch one extra to detect next page
	offset := int32(page * campaignListPageSize)
	items, err := deps.CampaignService.List(ctx, user.ID, limit, offset)
	if err != nil {
		deps.Log.Error("HandleCampaignList: list failed", zap.Error(err))
		replyText(api, update, "⚠️ Không tải được danh sách.")
		return nil
	}

	if len(items) == 0 && page == 0 {
		replyText(api, update, "Chưa có campaign nào. Tạo bằng /campaign new <money_url>.")
		return nil
	}

	hasNext := len(items) > campaignListPageSize
	if hasNext {
		items = items[:campaignListPageSize]
	}

	var b strings.Builder
	b.WriteString("📋 *Campaigns:*\n\n")
	for _, c := range items {
		b.WriteString(fmt.Sprintf("`%s` %s\n• %s • %d/%d credits\n\n",
			c.ID.String()[:8],
			escapeMD(c.Name),
			string(c.Status),
			c.CreditsConsumed,
			c.CreditsAllocated,
		))
	}

	chatID := updateChatID(update)
	msg := tgbotapi.NewMessage(chatID, b.String())
	msg.ParseMode = "Markdown"
	if page > 0 || hasNext {
		kb := buildPagerKeyboard("cmp:list", page, hasNext)
		msg.ReplyMarkup = kb
	}
	_, err = api.Send(msg)
	return err
}

// HandleCampaignListCallback handles "cmp:list:<page>" inline keyboard callbacks.
func HandleCampaignListCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	data := update.CallbackQuery.Data // "cmp:list:<page>"
	var page int
	fmt.Sscanf(strings.TrimPrefix(data, "cmp:list:"), "%d", &page)
	if page < 0 {
		page = 0
	}

	user, ok := UserFromCtx(ctx)
	if !ok {
		return nil
	}

	limit := int32(campaignListPageSize + 1)
	offset := int32(page * campaignListPageSize)
	items, err := deps.CampaignService.List(ctx, user.ID, limit, offset)
	if err != nil {
		deps.Log.Error("HandleCampaignListCallback: list failed", zap.Error(err))
		return nil
	}

	hasNext := len(items) > campaignListPageSize
	if hasNext {
		items = items[:campaignListPageSize]
	}

	var b strings.Builder
	b.WriteString("📋 *Campaigns:*\n\n")
	for _, c := range items {
		b.WriteString(fmt.Sprintf("`%s` %s\n• %s • %d/%d credits\n\n",
			c.ID.String()[:8],
			escapeMD(c.Name),
			string(c.Status),
			c.CreditsConsumed,
			c.CreditsAllocated,
		))
	}

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			b.String(),
		)
		edit.ParseMode = "Markdown"
		if page > 0 || hasNext {
			kb := buildPagerKeyboard("cmp:list", page, hasNext)
			edit.ReplyMarkup = &kb
		}
		_, _ = api.Send(edit)
	}
	return nil
}
