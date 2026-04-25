// cmd_download.go — Phase 07: /download command.
// Serves installer download URL from INSTALLER_URL env var.
// Empty URL → "installer not yet published" reply.
// Callback "download:guide" → installation guide text.
package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleDownload handles the /download command.
func HandleDownload(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)
	if chatID == 0 {
		return nil
	}

	url := ""
	if deps.Cfg != nil {
		url = deps.Cfg.InstallerURL
	}

	if url == "" {
		msg := tgbotapi.NewMessage(chatID,
			"Installer chưa publish. Theo dõi announcement nhé.")
		_, err := api.Send(msg)
		return err
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Hướng dẫn cài đặt", "download:guide"),
		),
	)

	text := fmt.Sprintf("📥 Tải installer Windows:\n%s", url)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	_, err := api.Send(msg)
	return err
}

// HandleDownloadGuideCallback handles "download:guide" — shows installation guide.
func HandleDownloadGuideCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}
	_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	guide := "📖 <b>Hướng dẫn cài đặt Snake Backlink Forge</b>\n\n" +
		"1. Tải file <code>.exe</code> từ link phía trên\n" +
		"2. Chạy installer với quyền <b>Run as Administrator</b>\n" +
		"3. Làm theo các bước trong wizard cài đặt\n" +
		"4. Mở ứng dụng → nhập <b>API key</b> từ /key\n" +
		"5. Bắt đầu chạy campaign từ giao diện chính\n\n" +
		"Nếu gặp sự cố, dùng /support để liên hệ đội kỹ thuật."

	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			guide,
		)
		edit.ParseMode = tgbotapi.ModeHTML
		_, _ = api.Send(edit)
	}
	return nil
}
