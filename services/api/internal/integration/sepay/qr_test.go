// Package sepay_test — unit tests for BuildQRURL (pure function, no DB/network).
package sepay_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
)

func TestBuildQRURL_ValidInputs_URLPrefix(t *testing.T) {
	result := sepay.BuildQRURL("MBBank", "1234567890", 329_000, "SBF TOPUP ABC123DEF456")
	if !strings.HasPrefix(result, "https://qr.sepay.vn/img?") {
		t.Fatalf("expected URL prefix https://qr.sepay.vn/img?, got %q", result)
	}
}

func TestBuildQRURL_AllParamsPresent(t *testing.T) {
	rawURL := sepay.BuildQRURL("MBBank", "1234567890", 329_000, "SBF TOPUP A1B2C3D4E5F6")

	// Strip scheme+host+path prefix to get query string.
	const prefix = "https://qr.sepay.vn/img?"
	if !strings.HasPrefix(rawURL, prefix) {
		t.Fatalf("unexpected URL: %s", rawURL)
	}
	qs, err := url.ParseQuery(rawURL[len(prefix):])
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}

	cases := []struct{ key, want string }{
		{"acc", "1234567890"},
		{"bank", "MBBank"},
		{"amount", "329000"},
		{"des", "SBF TOPUP A1B2C3D4E5F6"},
		{"template", "compact"},
	}
	for _, c := range cases {
		if got := qs.Get(c.key); got != c.want {
			t.Errorf("param %q: got %q, want %q", c.key, got, c.want)
		}
	}
}

func TestBuildQRURL_SpecialCharsInDes_URLEncoded(t *testing.T) {
	// "SBF TOPUP ABC12345" contains spaces — url.Values.Encode() converts them to '+'.
	rawURL := sepay.BuildQRURL("MBBank", "9876543210", 99_000, "SBF TOPUP ABC12345")

	// The raw URL string must contain the encoded form.
	if !strings.Contains(rawURL, "des=SBF+TOPUP+ABC12345") {
		t.Errorf("expected des=SBF+TOPUP+ABC12345 in URL, got: %s", rawURL)
	}

	// Parsed value must round-trip back to original (url.Values.Encode uses '+' for spaces).
	const prefix = "https://qr.sepay.vn/img?"
	qs, _ := url.ParseQuery(rawURL[len(prefix):])
	if got := qs.Get("des"); got != "SBF TOPUP ABC12345" {
		t.Errorf("decoded des: got %q, want %q", got, "SBF TOPUP ABC12345")
	}
}

func TestBuildQRURL_AmountDecimalInt(t *testing.T) {
	// Ensure large amounts are rendered as plain decimal integers (not scientific notation).
	rawURL := sepay.BuildQRURL("VCB", "0000111122", 2_399_000, "SBF TOPUP FFFFFFFFFFFF")

	const prefix = "https://qr.sepay.vn/img?"
	qs, _ := url.ParseQuery(rawURL[len(prefix):])
	if got := qs.Get("amount"); got != "2399000" {
		t.Errorf("amount: got %q, want %q", got, "2399000")
	}
}
