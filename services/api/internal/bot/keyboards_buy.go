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

// pkgLabel returns the display label for a package button.
// Featured packages get a ⭐ prefix.
func pkgLabel(code string) string {
	pkg, ok := service.Packages[code]
	if !ok {
		return code
	}
	label := pkg.DisplayVI
	if pkg.Featured {
		label = "⭐ " + label
	}
	return label
}

// pkgBtn is a shorthand for building a package selection button.
func pkgBtn(code string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(pkgLabel(code), "buy:pkg:"+code)
}

// PackageMenuKeyboard returns the 3-row inline keyboard for /buy.
// Row 1: Standard Starter / Basic / Pro* / Max
// Row 2: Premium Starter / Basic / Pro* / Max
// Row 3: Combo P100+S50 / Combo P200+S100
// lang is reserved for future i18n (currently uses DisplayVI for all).
func PackageMenuKeyboard(_ string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			pkgBtn("standard_starter_50"),
			pkgBtn("standard_basic_100"),
			pkgBtn("standard_pro_200"),
			pkgBtn("standard_max_300"),
		),
		tgbotapi.NewInlineKeyboardRow(
			pkgBtn("premium_starter_50"),
			pkgBtn("premium_basic_100"),
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
