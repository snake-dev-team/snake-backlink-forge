// cmd_admin.go — Phase 08: /admin command dispatcher.
// Subcmds: stats | grant | ban | unban | lookup.
// Non-admin callers are silently ignored (no info leak about admin namespace existence).
// Render helpers live in cmd_admin_render.go (same package).
package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// HandleAdmin routes /admin <subcmd> [args...] to the appropriate AdminService method.
// Silent ignore for non-admin callers — no reply, no error logged (no info leak).
func HandleAdmin(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}
	chatID := updateChatID(update)
	callerTGID := updateTelegramID(update)

	// Silent ignore for non-admins — do not reveal admin namespace.
	if !IsAdmin(deps.Cfg, callerTGID) {
		return nil
	}

	if deps.AdminService == nil {
		_, err := api.Send(tgbotapi.NewMessage(chatID, "Admin service not wired."))
		return err
	}

	args := strings.Fields(update.Message.Text)
	// args[0] = "/admin", args[1] = subcmd
	if len(args) < 2 {
		_, err := api.Send(tgbotapi.NewMessage(chatID, adminHelpText()))
		return err
	}

	switch args[1] {
	case "stats":
		return handleAdminStats(ctx, deps, api, chatID)
	case "grant":
		return handleAdminGrant(ctx, deps, api, chatID, callerTGID, args[2:])
	case "ban":
		return handleAdminBan(ctx, deps, api, chatID, callerTGID, args[2:])
	case "unban":
		return handleAdminUnban(ctx, deps, api, chatID, callerTGID, args[2:])
	case "lookup":
		return handleAdminLookup(ctx, deps, api, chatID, args[2:])
	default:
		_, err := api.Send(tgbotapi.NewMessage(chatID, adminHelpText()))
		return err
	}
}

// handleAdminStats renders /admin stats reply.
func handleAdminStats(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, chatID int64) error {
	st, err := deps.AdminService.Stats(ctx)
	if err != nil {
		deps.Log.Error("admin stats failed", zap.Error(err))
		_, sendErr := api.Send(tgbotapi.NewMessage(chatID, "Lỗi lấy stats: "+err.Error()))
		return sendErr
	}
	msg := tgbotapi.NewMessage(chatID, renderAdminStats(st))
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}

// handleAdminGrant handles: /admin grant <tg_id> <pool> <amount> [reason...]
func handleAdminGrant(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, chatID, callerTGID int64, args []string) error {
	if len(args) < 3 {
		_, err := api.Send(tgbotapi.NewMessage(chatID, "Dùng: /admin grant <tg_id> <pool> <amount> [reason...]"))
		return err
	}

	targetTGID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "tg_id không hợp lệ."))
		return err2
	}
	pool := args[1]
	amount, err := strconv.Atoi(args[2])
	if err != nil {
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "amount không hợp lệ."))
		return err2
	}
	reason := "manual"
	if len(args) > 3 {
		reason = strings.Join(args[3:], " ")
	}

	// Resolve target user UUID from tg_id via AdminService (keeps sqlc out of bot layer).
	targetUserID, resolveErr := deps.AdminService.GetUserByTGID(ctx, targetTGID)
	if resolveErr != nil {
		if errors.Is(resolveErr, service.ErrAdminUserNotFound) {
			_, err2 := api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Không tìm thấy user tg_id=%d.", targetTGID)))
			return err2
		}
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Lỗi: "+resolveErr.Error()))
		return err2
	}

	newBal, grantErr := deps.AdminService.Grant(ctx, callerTGID, targetUserID, pool, amount, reason)
	if grantErr != nil {
		switch {
		case errors.Is(grantErr, service.ErrAdminGrantInvalidPool):
			_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Pool phải là 'premium' hoặc 'standard'."))
			return err2
		case errors.Is(grantErr, service.ErrAdminGrantAmountOOB):
			_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Amount phải từ 1 đến 10000."))
			return err2
		default:
			deps.Log.Error("admin grant failed", zap.Int64("target_tg_id", targetTGID), zap.Error(grantErr))
			_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Grant thất bại: "+grantErr.Error()))
			return err2
		}
	}

	reply := fmt.Sprintf("Đã cộng *%d* %s credit cho tg_id=%d.\nSố dư mới: *%d*\nLý do: %s",
		amount, pool, targetTGID, newBal, reason)
	msg := tgbotapi.NewMessage(chatID, reply)
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}

// handleAdminBan handles: /admin ban <tg_id> [reason...]
func handleAdminBan(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, chatID, callerTGID int64, args []string) error {
	if len(args) < 1 {
		_, err := api.Send(tgbotapi.NewMessage(chatID, "Dùng: /admin ban <tg_id> [reason...]"))
		return err
	}
	targetTGID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "tg_id không hợp lệ."))
		return err2
	}
	reason := "admin_ban"
	if len(args) > 1 {
		reason = strings.Join(args[1:], " ")
	}

	if banErr := deps.AdminService.Ban(ctx, callerTGID, targetTGID, reason); banErr != nil {
		if errors.Is(banErr, service.ErrCannotBanAdmin) {
			// [M3] Do NOT leak which IDs are admins — generic refusal message only.
			_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Không thể ban tài khoản admin."))
			return err2
		}
		deps.Log.Error("admin ban failed", zap.Int64("target_tg_id", targetTGID), zap.Error(banErr))
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Ban thất bại: "+banErr.Error()))
		return err2
	}

	_, err = api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Đã ban tg_id=%d.", targetTGID)))
	return err
}

// handleAdminUnban handles: /admin unban <tg_id>
func handleAdminUnban(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, chatID, callerTGID int64, args []string) error {
	if len(args) < 1 {
		_, err := api.Send(tgbotapi.NewMessage(chatID, "Dùng: /admin unban <tg_id>"))
		return err
	}
	targetTGID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "tg_id không hợp lệ."))
		return err2
	}

	if unbanErr := deps.AdminService.Unban(ctx, callerTGID, targetTGID, "admin_unban"); unbanErr != nil {
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Unban thất bại: "+unbanErr.Error()))
		return err2
	}
	_, err = api.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Đã unban tg_id=%d.", targetTGID)))
	return err
}

// handleAdminLookup handles: /admin lookup <identifier>
func handleAdminLookup(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, chatID int64, args []string) error {
	if len(args) < 1 {
		_, err := api.Send(tgbotapi.NewMessage(chatID, "Dùng: /admin lookup <tg_id|+phone|sbf_live_...>"))
		return err
	}
	result, err := deps.AdminService.Lookup(ctx, args[0])
	if err != nil {
		if errors.Is(err, service.ErrAdminUserNotFound) {
			_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Không tìm thấy user."))
			return err2
		}
		_, err2 := api.Send(tgbotapi.NewMessage(chatID, "Lỗi: "+err.Error()))
		return err2
	}
	msg := tgbotapi.NewMessage(chatID, renderAdminLookup(result))
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}

// adminHelpText returns the /admin command menu.
func adminHelpText() string {
	return "*/admin* — lệnh quản trị:\n" +
		"`/admin stats` — thống kê tổng quan\n" +
		"`/admin grant <tg_id> <pool> <amount> [reason]` — cộng credit\n" +
		"`/admin ban <tg_id> [reason]` — khóa tài khoản\n" +
		"`/admin unban <tg_id>` — mở khóa\n" +
		"`/admin lookup <tg_id|+phone|key_prefix>` — tra cứu user"
}
