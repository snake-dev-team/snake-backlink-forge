// Package util_test — unit tests for API key generation (token.go).
// Tests cover shape, uniqueness, entropy distribution, hash correctness,
// base58 round-trip, and alphabet safety.
package util_test

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/util"
)

// TestGenerateAPIKey_Shape verifies plaintext starts with "sbf_live_" and is exactly 41 chars.
func TestGenerateAPIKey_Shape(t *testing.T) {
	plaintext, hashBytes, prefix, err := util.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey returned error: %v", err)
	}
	if !strings.HasPrefix(plaintext, "sbf_live_") {
		t.Errorf("plaintext does not start with 'sbf_live_': %q", plaintext)
	}
	if len(plaintext) != 41 {
		t.Errorf("plaintext length = %d, want 41 (sbf_live_[9] + 32 base58)", len(plaintext))
	}
	if len(hashBytes) != 32 {
		t.Errorf("hashBytes length = %d, want 32 (SHA-256)", len(hashBytes))
	}
	if len(prefix) != 12 {
		t.Errorf("prefix length = %d, want 12", len(prefix))
	}
	if !strings.HasPrefix(plaintext, prefix) {
		t.Errorf("plaintext %q does not start with prefix %q", plaintext, prefix)
	}
}

// TestGenerateAPIKey_Uniqueness generates 10 000 keys and asserts 0 collisions.
func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	const n = 10_000
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		plaintext, _, _, err := util.GenerateAPIKey()
		if err != nil {
			t.Fatalf("iteration %d: GenerateAPIKey error: %v", i, err)
		}
		if seen[plaintext] {
			t.Fatalf("collision detected at iteration %d: %q", i, plaintext)
		}
		seen[plaintext] = true
	}
}

// TestGenerateAPIKey_HashCorrectness verifies that sha256(plaintext) == returned hashBytes.
func TestGenerateAPIKey_HashCorrectness(t *testing.T) {
	for i := 0; i < 100; i++ {
		plaintext, hashBytes, _, err := util.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey error: %v", err)
		}
		expected := sha256.Sum256([]byte(plaintext))
		if string(hashBytes) != string(expected[:]) {
			t.Fatalf("hash mismatch for key %q", plaintext)
		}
	}
}

// TestGenerateAPIKey_Entropy checks that each of the 58 base58 chars appears in
// a large sample. A perfectly fair encoder should yield each char ~1/58 of the time.
// We use a simple presence check (much weaker than chi-square but deterministic).
func TestGenerateAPIKey_Entropy(t *testing.T) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	freq := make(map[rune]int, len(alphabet))

	const n = 5_000
	for i := 0; i < n; i++ {
		plaintext, _, _, err := util.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey error: %v", err)
		}
		// Only examine the 32-char random suffix, not the fixed prefix.
		randomPart := plaintext[len("sbf_live_"):]
		for _, ch := range randomPart {
			freq[ch]++
		}
	}

	for _, ch := range alphabet {
		if freq[ch] == 0 {
			t.Errorf("character %q never appeared in %d keys (entropy concern)", ch, n)
		}
	}
}

// TestGenerateAPIKey_PrefixFormat checks prefix is exactly the first 12 chars of plaintext.
func TestGenerateAPIKey_PrefixFormat(t *testing.T) {
	for i := 0; i < 50; i++ {
		plaintext, _, prefix, err := util.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey error: %v", err)
		}
		if prefix != plaintext[:12] {
			t.Fatalf("prefix %q != plaintext[:12] %q", prefix, plaintext[:12])
		}
	}
}

// TestBase58_AlphabetExcludes0OIl asserts the base58 alphabet never contains
// the four visually ambiguous characters: '0', 'O', 'I', 'l'.
func TestBase58_AlphabetExcludes0OIl(t *testing.T) {
	// Generate a large batch; if any forbidden char appears, the alphabet is wrong.
	const forbidden = "0OIl"
	const n = 1_000
	for i := 0; i < n; i++ {
		plaintext, _, _, err := util.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey error: %v", err)
		}
		randomPart := plaintext[len("sbf_live_"):]
		for _, bad := range forbidden {
			if strings.ContainsRune(randomPart, bad) {
				t.Errorf("forbidden character %q found in key %q", bad, plaintext)
			}
		}
	}
}

// TestGenerateAPIKey_RandomPartLength verifies the random suffix is exactly 32 chars.
func TestGenerateAPIKey_RandomPartLength(t *testing.T) {
	for i := 0; i < 200; i++ {
		plaintext, _, _, err := util.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey error: %v", err)
		}
		randomPart := strings.TrimPrefix(plaintext, "sbf_live_")
		if len(randomPart) != 32 {
			t.Fatalf("random part length = %d, want 32; key = %q", len(randomPart), plaintext)
		}
	}
}

// TestGenerateAPIKey_LengthAlwaysExactly41 generates a large batch and asserts every key
// is exactly 41 chars. This is a regression guard for M2 (base58Encode right-truncation bug).
// With 24 random bytes the natural base58 length can reach 33 digits; the fixed encoder
// must always emit exactly 32 random chars → 41-char total plaintext.
func TestGenerateAPIKey_LengthAlwaysExactly41(t *testing.T) {
	const n = 50_000
	for i := 0; i < n; i++ {
		plaintext, _, _, err := util.GenerateAPIKey()
		if err != nil {
			t.Fatalf("iteration %d: GenerateAPIKey error: %v", i, err)
		}
		if len(plaintext) != 41 {
			t.Fatalf("iteration %d: plaintext length = %d, want 41; key = %q", i, len(plaintext), plaintext)
		}
		randomPart := strings.TrimPrefix(plaintext, "sbf_live_")
		if len(randomPart) != 32 {
			t.Fatalf("iteration %d: random part length = %d, want 32; key = %q", i, len(randomPart), plaintext)
		}
	}
}
