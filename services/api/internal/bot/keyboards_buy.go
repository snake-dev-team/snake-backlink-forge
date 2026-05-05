// keyboards_buy.go — Phase 05: inline keyboard builders for /buy and /topup flows.
// Three keyboard types:
//   - PackageMenuKeyboard: 3-row grid of all 10 packages (Standard / Premium / Combo rows)
//   - ConfirmCancelKeyboard: confirm + cancel for a specific package before payment
//   - TopupActionsKeyboard: "Đã chuyển khoản" check + "Hủy" for the QR waiting screen
package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
)

// pkgLabel returns a compact button label that fits Telegram's 2-per-row width.
// Format: "<TierShort> <Tier suffix> <PriceCompact>" + featured ⭐ marker.
// e.g. "Std Starter 50 · 99K", "Prem Pro 200 ⭐ 1.7M", "Combo P100+S50 · 999K".
func pkgLabel(code string) string {
	pkg, ok := service.Packages[code]
	if !ok {
		return code
	}
	short := shortPkgName(code)
	star := ""
	if pkg.Featured {
		star = " ⭐"
	}
	return fmt.Sprintf("%s%s · %s", short, star, compactVND(pkg.AmountVND))
}

// shortPkgName collapses long DisplayVI into a Telegram-safe short form.
func shortPkgName(code string) string {
	// Map known codes to short forms; unknown codes fall back to code itself.
	switch code {
	case "standard_starter_50":
		return "Std Starter 50"
	case "standard_basic_100":
		return "Std Basic 100"
	case "standard_pro_200":
		return "Std Pro 200"
	case "standard_max_300":
		return "Std Max 300"
	case "premium_starter_50":
		return "Prem Starter 50"
	case "premium_basic_100":
		return "Prem Basic 100"
	case "premium_pro_200":
		return "Prem Pro 200"
	case "premium_max_300":
		return "Prem Max 300"
	case "combo_p100_s50":
		return "Combo P100+S50"
	case "combo_p200_s100":
		return "Combo P200+S100"
	}
	return code
}

// compactVND renders int64 VND as "99K" / "1.7M" for narrow keyboard buttons.
func compactVND(vnd int64) string {
	if vnd >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(vnd)/1_000_000)
	}
	return fmt.Sprintf("%dK", vnd/1000)
}

// pkgBtn is a shorthand for building a package selection button.
func pkgBtn(code string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(pkgLabel(code), "buy:pkg:"+code)
}

// PackageMenuKeyboard returns the 5-row × 2-button-per-row inline keyboard for /buy.
// 2-per-row keeps button labels readable on Telegram (4-per-row truncates).
// Spec §5.1 + §1.3: 2 rows Standard + 2 rows Premium + 1 row Combo.
// lang is reserved for future i18n (currently uses DisplayVI for all).
func PackageMenuKeyboard(_ string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			pkgBtn("standard_starter_50"),
			pkgBtn("standard_basic_100"),
		),
		tgbotapi.NewInlineKeyboardRow(
			pkgBtn("standard_pro_200"),
			pkgBtn("standard_max_300"),
		),
		tgbotapi.NewInlineKeyboardRow(
			pkgBtn("premium_starter_50"),
			pkgBtn("premium_basic_100"),
		),
		tgbotapi.NewInlineKeyboardRow(
			pkgBtn("premium_pro_200"),
			pkgBtn("premium_max_300"),
		),
		tgbotapi.NewInlineKeyboardRow(
			pkgBtn("combo_p100_s50"),
			pkgBtn("combo_p200_s100"),
		),
	)
}

// ConfirmCancelKeyboard returns the confirm/cancel keyboard shown after a package is selected.
// Confirm callback: "buy:confirm:<pkgCode>"
// Cancel callback:  "buy:cancel"
func ConfirmCancelKeyboard(pkgCode string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Xác nhận", "buy:confirm:"+pkgCode),
			tgbotapi.NewInlineKeyboardButtonData("❌ Huỷ", "buy:cancel"),
		),
	)
}

// TopupActionsKeyboard returns the action buttons shown below the QR code image.
// "Đã chuyển khoản" polls status. "Hủy" cancels the pending transaction.
func TopupActionsKeyboard(txID uuid.UUID) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"✅ Đã chuyển khoản",
				fmt.Sprintf("topup:check:%s", txID),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				"❌ Hủy",
				fmt.Sprintf("topup:cancel:%s", txID),
			),
		),
	)
}
