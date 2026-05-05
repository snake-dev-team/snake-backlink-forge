// verify_test.go — unit tests for VerifyApikey; no DB, no network.
package sepay

import "testing"

func TestVerifyApikey_Match(t *testing.T) {
	if !VerifyApikey("Apikey mysecrettoken", "mysecrettoken") {
		t.Fatal("expected true for correct token + prefix")
	}
}

func TestVerifyApikey_Mismatch(t *testing.T) {
	if VerifyApikey("Apikey wrongtoken", "mysecrettoken") {
		t.Fatal("expected false for mismatched token")
	}
}

func TestVerifyApikey_NoPrefix(t *testing.T) {
	if VerifyApikey("Bearer mysecrettoken", "mysecrettoken") {
		t.Fatal("expected false for Bearer prefix (not Apikey)")
	}
}

func TestVerifyApikey_Empty(t *testing.T) {
	if VerifyApikey("", "mysecrettoken") {
		t.Fatal("expected false for empty header")
	}
}

func TestVerifyApikey_EmptyExpected(t *testing.T) {
	// Fail-closed: empty expected MUST return false regardless of header value.
	// Prevents a misconfigured server (SEPAY_WEBHOOK_TOKEN="") from accepting all requests.
	if VerifyApikey("Apikey anything", "") {
		t.Fatal("expected false when expected is empty — fail-closed against misconfig")
	}
}

func TestVerifyApikey_ConstantTime(t *testing.T) {
	// Loose timing test: verifies the function runs to completion on identical-length wrong tokens.
	// True constant-time measurement is not achievable in unit tests (no ns precision guarantee).
	// This test validates correctness only — the crypto/subtle.ConstantTimeCompare call itself
	// provides the timing guarantee at the implementation level.
	got := VerifyApikey("Apikey AAAAAAAAAAAA", "BBBBBBBBBBBB") // same length, wrong
	if got {
		t.Fatal("expected false for same-length wrong token")
	}
}
