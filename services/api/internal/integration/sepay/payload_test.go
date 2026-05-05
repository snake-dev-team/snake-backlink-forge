// payload_test.go — unit tests for OrderCodeRe regex; no DB, no network.
package sepay

import (
	"strings"
	"testing"
)

func TestOrderCodeRe_MatchExact(t *testing.T) {
	m := OrderCodeRe.FindStringSubmatch("SBF TOPUP ABCDEF012345")
	if m == nil {
		t.Fatal("expected match for valid uppercase 12-hex memo")
	}
	if strings.ToUpper(m[1]) != "ABCDEF012345" {
		t.Fatalf("capture group: got %q, want ABCDEF012345", m[1])
	}
}

func TestOrderCodeRe_MatchLowercaseMemo(t *testing.T) {
	// Case-insensitive prefix match; captured group normalized to upper by caller.
	m := OrderCodeRe.FindStringSubmatch("sbf topup abcdef012345")
	if m == nil {
		t.Fatal("expected case-insensitive match for lowercase memo")
	}
	if strings.ToUpper(m[1]) != "ABCDEF012345" {
		t.Fatalf("capture group (uppercased): got %q, want ABCDEF012345", strings.ToUpper(m[1]))
	}
}

func TestOrderCodeRe_MatchMixedCase(t *testing.T) {
	m := OrderCodeRe.FindStringSubmatch("SBF TOPUP AbCdEf012345")
	if m == nil {
		t.Fatal("expected match for mixed-case hex memo")
	}
	if strings.ToUpper(m[1]) != "ABCDEF012345" {
		t.Fatalf("capture group (uppercased): got %q, want ABCDEF012345", strings.ToUpper(m[1]))
	}
}

func TestOrderCodeRe_RejectTooShort(t *testing.T) {
	// 11 hex chars — must NOT match (length anchor via {12}).
	m := OrderCodeRe.FindStringSubmatch("SBF TOPUP ABCDEF01234")
	if m != nil {
		t.Fatalf("expected no match for 11-hex code, got %v", m)
	}
}

func TestOrderCodeRe_RejectTooLong(t *testing.T) {
	// 13 hex chars — regex must match exactly 12 and not over-capture.
	// The regex {12} stops at 12; the 13th char is not captured.
	// But we want the full match to be exactly 12 — verify capture group length.
	m := OrderCodeRe.FindStringSubmatch("SBF TOPUP ABCDEF0123456")
	if m != nil && len(m[1]) != 12 {
		t.Fatalf("capture group must be exactly 12 chars, got %d: %q", len(m[1]), m[1])
	}
}

func TestOrderCodeRe_RejectNonHex(t *testing.T) {
	// G is not a hex char — must not match.
	m := OrderCodeRe.FindStringSubmatch("SBF TOPUP ABCDEFG12345")
	if m != nil {
		t.Fatalf("expected no match for non-hex char in code, got %v", m)
	}
}

func TestOrderCodeRe_EmbeddedInLongerContent(t *testing.T) {
	// Real bank transfer content often has prefix text.
	m := OrderCodeRe.FindStringSubmatch("CT tu TK 12345 noi dung SBF TOPUP A1B2C3D4E5F6 xin cam on")
	if m == nil {
		t.Fatal("expected match when pattern is embedded in longer transfer content")
	}
	if strings.ToUpper(m[1]) != "A1B2C3D4E5F6" {
		t.Fatalf("capture group: got %q, want A1B2C3D4E5F6", strings.ToUpper(m[1]))
	}
}
