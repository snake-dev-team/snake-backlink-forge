// Package util — E.164 phone number normalizer with VN default country.
// Uses github.com/nyaruka/phonenumbers (libphonenumber port) with a regex
// fast-path for common Vietnamese formats so we avoid CGO overhead on the hot path.
package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

const defaultRegion = "VN"

// vnFastPath matches the two most common raw formats from Telegram contacts:
//   - 0xxxxxxxxx  (10 digits, starts with 0)    → +84xxxxxxxxx
//   - 84xxxxxxxxx (11 digits, starts with 84)   → +84xxxxxxxxx
//
// It does NOT match +84... (already has country code prefix) which falls through
// to the libphonenumber parse.  Strip spaces/dashes/parens before matching.
var vnFastPath = regexp.MustCompile(`^(?:0|84)(\d{9,10})$`)

// NormalizePhone converts rawPhone to E.164 format (+84...).
// Strips spaces, dashes, and parentheses before parsing.
// Rejects strings containing letters (e.g. vanity numbers like 098ABC4321).
// Returns an error if the number cannot be parsed or is invalid for VN.
func NormalizePhone(rawPhone string) (string, error) {
	// Reject strings with non-phone characters (letters).
	// Check BEFORE stripping so we catch mixed-format like "098ABC4321".
	trimmed := strings.TrimSpace(rawPhone)
	if trimmed == "" {
		return "", fmt.Errorf("phone: empty input")
	}
	if !allowedPhoneChars.MatchString(trimmed) {
		return "", fmt.Errorf("phone: contains invalid characters %q", safeLog(rawPhone))
	}

	// Strip formatting characters that Telegram or users might include.
	cleaned := stripFormatting(rawPhone)
	if cleaned == "" {
		return "", fmt.Errorf("phone: empty after stripping formatting")
	}

	// Fast path: common VN local formats without country code prefix.
	if m := vnFastPath.FindStringSubmatch(cleaned); m != nil {
		candidate := "+84" + m[1]
		// Validate through libphonenumber to confirm it's a real VN number.
		return validateE164(candidate)
	}

	// General path: let libphonenumber parse with VN as default region.
	// This handles +84..., +1..., international formats, etc.
	num, err := phonenumbers.Parse(cleaned, defaultRegion)
	if err != nil {
		return "", fmt.Errorf("phone: parse %q: %w", safeLog(rawPhone), err)
	}

	if !phonenumbers.IsValidNumber(num) {
		return "", fmt.Errorf("phone: invalid number %q", safeLog(rawPhone))
	}

	return phonenumbers.Format(num, phonenumbers.E164), nil
}

// validateE164 parses an already-prefixed E.164 string and re-formats it to
// canonical form, confirming libphonenumber considers it valid.
func validateE164(e164 string) (string, error) {
	num, err := phonenumbers.Parse(e164, "")
	if err != nil {
		return "", fmt.Errorf("phone: validate %q: %w", safeLog(e164), err)
	}
	if !phonenumbers.IsValidNumber(num) {
		return "", fmt.Errorf("phone: invalid number %q", safeLog(e164))
	}
	return phonenumbers.Format(num, phonenumbers.E164), nil
}

// allowedPhoneChars matches strings that contain ONLY valid phone chars after trim.
// Allowed: digits, +, spaces, dashes, dots, parens.
// Letters (e.g. 098ABC4321) are rejected — libphonenumber maps them via keypad which
// we do not want to silently accept as phone numbers from Telegram contacts.
var allowedPhoneChars = regexp.MustCompile(`^[0-9+\s\-.()()]+$`)

// stripFormatting removes spaces, dashes, dots, and parentheses.
// Returns error if the string contains disallowed characters (letters, etc.).
func stripFormatting(s string) string {
	s = strings.TrimSpace(s)
	r := strings.NewReplacer(" ", "", "-", "", ".", "", "(", "", ")", "")
	return r.Replace(s)
}

// safeLog returns the first 6 chars of a phone string for log output — never
// logs a full number to avoid PII in log streams.
func safeLog(s string) string {
	if len(s) <= 6 {
		return "***"
	}
	return s[:6] + "***"
}
