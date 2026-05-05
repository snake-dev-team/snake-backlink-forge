// cmd_campaign_jobs.go — Phase 7.02: /campaign jobs <id_prefix> handler + pagination.
// Shows up to 10 jobs per page for a campaign, ordered by created_at DESC.
package bot

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

const campaignJobsPageSize = 10

// jobStatusEmoji maps a job status to a display emoji.
func jobStatusEmoji(s sqlcdb.JobStatus) string {
	switch s {
	case sqlcdb.JobStatusQueued:
		return "⏳"
	case sqlcdb.JobStatusDispatched:
		return "🚀"
	case sqlcdb.JobStatusInProgress:
		return "⚙️"
	case sqlcdb.JobStatusSuccess:
		return "✅"
	case sqlcdb.JobStatusFailed:
		return "❌"
	case sqlcdb.JobStatusSkipped:
		return "⏭️"
	default:
		return "❓"
	}
}

// targetDomain extracts hostname from a URL string; returns the raw string on parse failure.
func targetDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Hostname()
}

// HandleCampaignJobs handles /campaign jobs <id_prefix> [page].
// args[0] is the ID prefix; page is 0-indexed.
func HandleCampaignJobs(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update, args []string, page int) error {
	user, ok := UserFromCtx(ctx)
	if !ok {
		replyText(api, update, "⚠️ Vui lòng /start trước.")
		return nil
	}
	if len(args) == 0 {
		replyText(api, update, "Cú pháp: /campaign jobs <id>")
		return nil
	}

	campaignID, resolveErr := resolveCampaignID(ctx, deps, user.ID, args[0])
	if resolveErr != nil {
		replyText(api, update, resolveErr.Error())
		return nil
	}

	text, hasNext, err := buildJobsMessage(ctx, deps, user.ID, campaignID, page)
	if err != nil {
		deps.Log.Error("HandleCampaignJobs: list failed", zap.Error(err))
		replyText(api, update, "⚠️ Không tải được jobs.")
		return nil
	}

	chatID := updateChatID(update)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	pagerPrefix := fmt.Sprintf("cmp:jobs:%s", campaignID.String()[:8])
	if page > 0 || hasNext {
		kb := buildPagerKeyboard(pagerPrefix, page, hasNext)
		msg.ReplyMarkup = kb
	}
	_, err = api.Send(msg)
	return err
}

// HandleCampaignJobsCallback handles "cmp:jobs:<id8>:<page>" callbacks.
func HandleCampaignJobsCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	user, ok := UserFromCtx(ctx)
	if !ok {
		return nil
	}

	// data format: "cmp:jobs:<id8>:<page>"
	data := strings.TrimPrefix(update.CallbackQuery.Data, "cmp:jobs:")
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	idPrefix := parts[0]
	var page int
	fmt.Sscanf(parts[1], "%d", &page)
	if page < 0 {
		page = 0
	}

	campaignID, resolveErr := resolveCampaignID(ctx, deps, user.ID, idPrefix)
	if resolveErr != nil {
		return nil
	}

	text, hasNext, err := buildJobsMessage(ctx, deps, user.ID, campaignID, page)
	if err != nil {
		deps.Log.Error("HandleCampaignJobsCallback: list failed", zap.Error(err))
		return nil
	}

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			text,
		)
		edit.ParseMode = "Markdown"
		pagerPrefix := fmt.Sprintf("cmp:jobs:%s", campaignID.String()[:8])
		if page > 0 || hasNext {
			kb := buildPagerKeyboard(pagerPrefix, page, hasNext)
			edit.ReplyMarkup = &kb
		}
		_, _ = api.Send(edit)
	}
	return nil
}

// buildJobsMessage fetches one page of jobs and renders the message text.
// Returns (text, hasNextPage, error).
func buildJobsMessage(ctx context.Context, deps *Deps, userID, campaignID uuid.UUID, page int) (string, bool, error) {
	limit := int32(campaignJobsPageSize + 1)
	offset := int32(page * campaignJobsPageSize)
	jobs, err := deps.JobService.ListByCampaign(ctx, userID, campaignID, limit, offset)
	if err != nil {
		return "", false, err
	}

	hasNext := len(jobs) > campaignJobsPageSize
	if hasNext {
		jobs = jobs[:campaignJobsPageSize]
	}

	if len(jobs) == 0 && page == 0 {
		return "Chưa có job nào cho campaign này.", false, nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("⚙️ *Jobs* (trang %d):\n\n", page+1))
	for _, j := range jobs {
		icon := jobStatusEmoji(j.Status)
		domain := escapeMD(targetDomain(j.TargetUrlSnapshot))
		anchor := escapeMD(j.AnchorText)
		line := fmt.Sprintf("%s %s • %s", icon, domain, anchor)
		if j.ErrorCode != nil && *j.ErrorCode != "" {
			line += fmt.Sprintf(" [%s]", escapeMD(*j.ErrorCode))
		}
		b.WriteString(line + "\n")
	}
	return b.String(), hasNext, nil
}
