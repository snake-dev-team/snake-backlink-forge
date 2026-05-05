// cmd_campaign_status.go — Phase 7.02: /campaign pause|resume|archive handler.
// Shared handler for all three status transitions; action string drives the switch.
package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
)

// HandleCampaignStatus handles /campaign pause|resume|archive <id_prefix>.
// action must be one of "pause", "resume", "archive".
func HandleCampaignStatus(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update, action string, args []string) error {
	user, ok := UserFromCtx(ctx)
	if !ok {
		replyText(api, update, "⚠️ Vui lòng /start trước.")
		return nil
	}
	if len(args) == 0 {
		replyText(api, update, fmt.Sprintf("Cú pháp: /campaign %s <id>", action))
		return nil
	}

	id, resolveErr := resolveCampaignID(ctx, deps, user.ID, args[0])
	if resolveErr != nil {
		replyText(api, update, resolveErr.Error())
		return nil
	}

	var status sqlcdb.CampaignStatus
	switch action {
	case "pause":
		status = sqlcdb.CampaignStatusPaused
	case "resume":
		status = sqlcdb.CampaignStatusRunning
	case "archive":
		status = sqlcdb.CampaignStatusArchived
	default:
		replyText(api, update, "Action không hợp lệ.")
		return nil
	}

	c, err := deps.CampaignService.SetStatus(ctx, user.ID, id, status)
	if err != nil {
		replyText(api, update, formatCampaignErr(err))
		return nil
	}

	text := fmt.Sprintf("✅ %s → *%s*", escapeMD(c.Name), string(c.Status))
	msg := tgbotapi.NewMessage(updateChatID(update), text)
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}
