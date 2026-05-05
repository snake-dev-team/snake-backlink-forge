// cmd_campaign_stats.go — Phase 7.02: /campaign stats <id_prefix> handler.
// Renders aggregated job KPIs for a campaign using the CampaignJobStats sqlc query.
package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// HandleCampaignStats handles /campaign stats <id_prefix>.
// Fetches campaign meta (name, credits) + job stats, renders a compact KPI table.
func HandleCampaignStats(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update, args []string) error {
	user, ok := UserFromCtx(ctx)
	if !ok {
		replyText(api, update, "⚠️ Vui lòng /start trước.")
		return nil
	}
	if len(args) == 0 {
		replyText(api, update, "Cú pháp: /campaign stats <id>")
		return nil
	}

	id, resolveErr := resolveCampaignID(ctx, deps, user.ID, args[0])
	if resolveErr != nil {
		replyText(api, update, resolveErr.Error())
		return nil
	}

	c, err := deps.CampaignService.Get(ctx, user.ID, id)
	if err != nil {
		replyText(api, update, formatCampaignErr(err))
		return nil
	}

	stats, err := deps.JobService.GetCampaignJobStats(ctx, user.ID, id)
	if err != nil {
		deps.Log.Error("HandleCampaignStats: GetCampaignJobStats failed",
			zap.String("campaign_id", id.String()), zap.Error(err))
		replyText(api, update, "⚠️ Không tải được KPI.")
		return nil
	}

	text := fmt.Sprintf(
		"📊 *%s*\nTotal: %d\n• ⏳ Queued: %d\n• 🚀 Dispatched: %d\n• ⚙️ In progress: %d\n• ✅ Success: %d\n• ❌ Failed: %d\n• ⏭️ Skipped: %d\nCredits: %d/%d",
		escapeMD(c.Name),
		stats.Total,
		stats.Queued,
		stats.Dispatched,
		stats.InProgress,
		stats.Success,
		stats.Failed,
		stats.Skipped,
		c.CreditsConsumed,
		c.CreditsAllocated,
	)

	msg := tgbotapi.NewMessage(updateChatID(update), text)
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}
