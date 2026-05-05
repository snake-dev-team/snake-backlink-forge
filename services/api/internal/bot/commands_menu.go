// commands_menu.go — Telegram bot command list shown in the "Menu" button
// next to the message input.
//
// Public commands only. /admin is intentionally excluded — it's gated by
// ADMIN_TELEGRAM_IDS check and exposing it via menu would leak admin
// existence to every user. /topup is part of the /buy flow (not a standalone
// entry point). /ping is debug-only.
package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// publicCommandsVI is the default (Vietnamese) menu — primary market.
var publicCommandsVI = []tgbotapi.BotCommand{
	{Command: "start", Description: "Bắt đầu / xác minh số điện thoại"},
	{Command: "balance", Description: "Xem số dư credits"},
	{Command: "buy", Description: "Mua gói credits"},
	{Command: "history", Description: "Lịch sử giao dịch"},
	{Command: "campaign", Description: "Quản lý campaigns (list/new/pause/resume/archive/jobs/stats)"},
	{Command: "key", Description: "Xem API key"},
	{Command: "regenkey", Description: "Tạo lại API key"},
	{Command: "support", Description: "Liên hệ hỗ trợ"},
	{Command: "download", Description: "Tải installer"},
	{Command: "ref", Description: "Mã giới thiệu"},
	{Command: "language", Description: "Đổi ngôn ngữ / Change language"},
}

// publicCommandsEN is served to users with Telegram language_code "en".
// Telegram auto-falls back to the default (VI) list if no en override exists.
var publicCommandsEN = []tgbotapi.BotCommand{
	{Command: "start", Description: "Start / verify phone number"},
	{Command: "balance", Description: "Show credit balance"},
	{Command: "buy", Description: "Buy a credit package"},
	{Command: "history", Description: "Transaction history"},
	{Command: "campaign", Description: "Manage campaigns (list/new/pause/resume/archive/jobs/stats)"},
	{Command: "key", Description: "Show API key"},
	{Command: "regenkey", Description: "Rotate API key"},
	{Command: "support", Description: "Contact support"},
	{Command: "download", Description: "Download installer"},
	{Command: "ref", Description: "Referral code"},
	{Command: "language", Description: "Change language / Đổi ngôn ngữ"},
}

// registerCommandMenu publishes the bot command list to Telegram. The list
// is what users see when they tap the "Menu" button next to the message bar.
// Failures are logged at Warn — commands remain callable even if menu publish
// fails, so this never blocks bot startup.
func registerCommandMenu(api *tgbotapi.BotAPI, log *zap.Logger) {
	if _, err := api.Request(tgbotapi.NewSetMyCommands(publicCommandsVI...)); err != nil {
		log.Warn("bot: setMyCommands default(VI) failed", zap.Error(err))
	}
	enReq := tgbotapi.NewSetMyCommandsWithScopeAndLanguage(
		tgbotapi.NewBotCommandScopeDefault(), "en", publicCommandsEN...,
	)
	if _, err := api.Request(enReq); err != nil {
		log.Warn("bot: setMyCommands EN failed", zap.Error(err))
	}
}
