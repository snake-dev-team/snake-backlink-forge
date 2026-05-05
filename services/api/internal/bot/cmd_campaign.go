// cmd_campaign.go — Phase 7.02: top-level /campaign dispatcher.
// Parses the first argument as a subcommand and routes to subhandlers.
// All subhandlers are in separate files (cmd_campaign_*.go).
package bot

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleCampaign is the entry point for the /campaign command.
// Nil-guards CampaignService + JobService so missing wiring never panics.
func HandleCampaign(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if deps.CampaignService == nil || deps.JobService == nil {
		replyText(api, update, "Tính năng campaign chưa sẵn sàng.")
		return nil
	}
	args := strings.Fields(update.Message.CommandArguments())
	if len(args) == 0 {
		return cmdCampaignHelp(api, update)
	}
	sub := strings.ToLower(args[0])
	rest := args[1:]
	switch sub {
	case "list":
		return HandleCampaignList(ctx, deps, api, update, 0)
	case "new":
		return HandleCampaignNew(ctx, deps, api, update, rest)
	case "pause", "resume", "archive":
		return HandleCampaignStatus(ctx, deps, api, update, sub, rest)
	case "jobs":
		return HandleCampaignJobs(ctx, deps, api, update, rest, 0)
	case "stats":
		return HandleCampaignStats(ctx, deps, api, update, rest)
	default:
		return cmdCampaignHelp(api, update)
	}
}

// cmdCampaignHelp sends the /campaign usage guide.
func cmdCampaignHelp(api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	text := "Campaign commands:\n" +
		"/campaign list — Liệt kê campaigns\n" +
		"/campaign new <money_url> — Tạo nhanh\n" +
		"/campaign pause <id> — Tạm dừng\n" +
		"/campaign resume <id> — Chạy tiếp\n" +
		"/campaign archive <id> — Lưu trữ\n" +
		"/campaign jobs <id> — Xem jobs\n" +
		"/campaign stats <id> — Xem KPI"
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	_, err := api.Send(msg)
	return err
}
