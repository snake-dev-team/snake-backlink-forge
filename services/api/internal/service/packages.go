// packages.go — Phase 05: static package registry (single source of truth).
// Used by TransactionService (snapshot at INSERT time) and bot buy keyboard.
// No DB table — changes go through code review; rare pricing updates are intentional.
package service

import (
	"fmt"
	"regexp"
)

// Package describes a purchasable credit bundle.
// AmountVND + PremiumCredits + StandardGranted are snapshotted into the
// transactions row at INSERT time — webhook reads from row, not this map.
type Package struct {
	Code            string
	DisplayVI       string
	DisplayEN       string
	AmountVND       int64
	PremiumCredits  int
	StandardCredits int
	Featured        bool // shown with ⭐ in UI
}

// Packages is the authoritative package registry (10 bundles per MASTER_PROMPT §1.3).
var Packages = map[string]Package{
	"standard_starter_50": {
		Code: "standard_starter_50", AmountVND: 99_000, StandardCredits: 50,
		DisplayVI: "Standard Starter — 50 credit", DisplayEN: "Standard Starter — 50 credits",
	},
	"standard_basic_100": {
		Code: "standard_basic_100", AmountVND: 179_000, StandardCredits: 100,
		DisplayVI: "Standard Basic — 100 credit", DisplayEN: "Standard Basic — 100 credits",
	},
	"standard_pro_200": {
		Code: "standard_pro_200", AmountVND: 329_000, StandardCredits: 200, Featured: true,
		DisplayVI: "Standard Pro — 200 credit", DisplayEN: "Standard Pro — 200 credits",
	},
	"standard_max_300": {
		Code: "standard_max_300", AmountVND: 459_000, StandardCredits: 300,
		DisplayVI: "Standard Max — 300 credit", DisplayEN: "Standard Max — 300 credits",
	},
	"premium_starter_50": {
		Code: "premium_starter_50", AmountVND: 499_000, PremiumCredits: 50,
		DisplayVI: "Premium Starter — 50 credit", DisplayEN: "Premium Starter — 50 credits",
	},
	"premium_basic_100": {
		Code: "premium_basic_100", AmountVND: 899_000, PremiumCredits: 100,
		DisplayVI: "Premium Basic — 100 credit", DisplayEN: "Premium Basic — 100 credits",
	},
	"premium_pro_200": {
		Code: "premium_pro_200", AmountVND: 1_699_000, PremiumCredits: 200, Featured: true,
		DisplayVI: "Premium Pro — 200 credit", DisplayEN: "Premium Pro — 200 credits",
	},
	"premium_max_300": {
		Code: "premium_max_300", AmountVND: 2_399_000, PremiumCredits: 300,
		DisplayVI: "Premium Max — 300 credit", DisplayEN: "Premium Max — 300 credits",
	},
	"combo_p100_s50": {
		Code: "combo_p100_s50", AmountVND: 999_000, PremiumCredits: 100, StandardCredits: 50,
		DisplayVI: "Combo 100P+50S", DisplayEN: "Combo 100 Premium + 50 Standard",
	},
	"combo_p200_s100": {
		Code: "combo_p200_s100", AmountVND: 1_799_000, PremiumCredits: 200, StandardCredits: 100,
		DisplayVI: "Combo 200P+100S", DisplayEN: "Combo 200 Premium + 100 Standard",
	},
}

// pkgCodeRegex is the defense-in-depth guard against malformed/injected package codes.
// Accepted: standard/premium_<tier>_<credits> OR combo_p<n>_s<n>.
var pkgCodeRegex = regexp.MustCompile(
	`^(standard|premium)_(starter|basic|pro|max)_(50|100|200|300)$|^combo_(p100_s50|p200_s100)$`,
)

// ValidatePackageCode returns a non-nil error if the code is not in the Packages map
// OR fails the defense-in-depth regex check. Both guards must pass.
func ValidatePackageCode(code string) error {
	if !pkgCodeRegex.MatchString(code) {
		return fmt.Errorf("unknown_package: %s", code)
	}
	if _, ok := Packages[code]; !ok {
		return fmt.Errorf("unknown_package: %s", code)
	}
	return nil
}
