// cmd_campaign_new.go — Phase 7.02: /campaign new <money_url> handler.
// Quick-creates a campaign with sensible defaults. Per-update serialization
// is already enforced by Bot.handleUpdate (per-user mutex), so no extra lock needed.
package bot

import (
	"context"
	"fmt"
	"net/url"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
)

// HandleCampaignNew handles /campaign new <money_url>.
// Defaults: pool=standard, source_mode=prebuilt, daily_limit=5,
// credits_allocated=10, start_now=false (draft).
func HandleCampaignNew(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update, args []string) error {
	user, ok := UserFromCtx(ctx)
	if !ok {
		replyText(api, update, "⚠️ Vui lòng /start trước.")
		return nil
	}
	if len(args) == 0 {
		replyText(api, update, "Cú pháp: /campaign new <money_url>")
		return nil
	}

	moneyURL := args[0]
	parsed, err := url.Parse(moneyURL)
	if err != nil || parsed.Host == "" {
		replyText(api, update, "URL không hợp lệ. Ví dụ: /campaign new https://example.com")
		return nil
	}

	hostname := parsed.Hostname()
	name := hostname
	if len(name) > 30 {
		name = name[:30]
	}

	in := service.CampaignInput{
		Name:             name,
		MoneySiteURL:     moneyURL,
		NicheKeywords:    []string{},
		AnchorTexts:      []service.AnchorTextInput{{Text: hostname, Type: "branded", Weight: 1}},
		Pool:             "standard",
		SourceMode:       "prebuilt",
		DailyLimit:       5,
		CreditsAllocated: 10,
		EthicalMode:      true,
		NicheFilter:      true,
		StartNow:         false,
	}

	c, err := deps.CampaignService.Create(ctx, user.ID, in)
	if err != nil {
		replyText(api, update, formatCampaignErr(err))
		return nil
	}

	text := fmt.Sprintf(
		"✅ Campaign tạo: `%s` %s\nDùng /campaign jobs %s để xem jobs.",
		c.ID.String()[:8],
		escapeMD(c.Name),
		c.ID.String()[:8],
	)
	msg := tgbotapi.NewMessage(updateChatID(update), text)
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}
