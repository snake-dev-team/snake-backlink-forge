// redact_test.go — unit tests for PII-hashing helpers in sepay package.
package sepay

import (
	"encoding/hex"
	"testing"
)

func TestHashIP_Deterministic(t *testing.T) {
	// Same input must always produce the same hash.
	ip := "203.0.113.42"
	h1 := HashIP(ip)
	h2 := HashIP(ip)
	if h1 != h2 {
		t.Fatalf("HashIP not deterministic: %q vs %q", h1, h2)
	}
}

func TestHashIP_Length(t *testing.T) {
	// Full SHA-256 → 32 bytes → 64 hex chars.
	h := HashIP("1.2.3.4")
	if len(h) != 64 {
		t.Fatalf("HashIP: want 64-char hex, got %d chars: %q", len(h), h)
	}
	if _, err := hex.DecodeString(h); err != nil {
		t.Fatalf("HashIP: not valid hex: %v", err)
	}
}

func TestHashIP_DifferentInputsDifferentHashes(t *testing.T) {
	h1 := HashIP("192.168.1.1")
	h2 := HashIP("10.0.0.1")
	if h1 == h2 {
		t.Fatal("HashIP: distinct IPs must not produce the same hash")
	}
}

func TestHashAccountPrefix_Length(t *testing.T) {
	// First 12 hex chars of SHA-256.
	h := HashAccountPrefix("1234567890")
	if len(h) != 12 {
		t.Fatalf("HashAccountPrefix: want 12-char hex, got %d chars: %q", len(h), h)
	}
	if _, err := hex.DecodeString(h); err != nil {
		t.Fatalf("HashAccountPrefix: not valid hex: %v", err)
	}
}

func TestHashAccountPrefix_Deterministic(t *testing.T) {
	acc := "0987654321"
	h1 := HashAccountPrefix(acc)
	h2 := HashAccountPrefix(acc)
	if h1 != h2 {
		t.Fatalf("HashAccountPrefix not deterministic: %q vs %q", h1, h2)
	}
}

func TestHashAccountPrefix_IsPrefixOfFullHash(t *testing.T) {
	// The 12-char prefix must be the first 12 chars of the full hash.
	acc := "VCB123456789"
	fullHash := HashIP(acc) // HashIP and HashAccountPrefix use same algorithm on string
	// Recompute via HashAccountPrefix
	prefix := HashAccountPrefix(acc)
	if fullHash[:12] != prefix {
		t.Fatalf("HashAccountPrefix must be first 12 chars of full hash: full=%s prefix=%s", fullHash[:12], prefix)
	}
}
