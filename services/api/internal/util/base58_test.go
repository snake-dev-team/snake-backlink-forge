// base58_test.go — unit tests for GenBase58Upper (referral code generator).
package util_test

import (
	"strings"
	"testing"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/util"
)

// TestGenBase58Upper_Length verifies that GenBase58Upper(n) always returns exactly n chars.
func TestGenBase58Upper_Length(t *testing.T) {
	for _, n := range []int{1, 6, 8, 16, 32} {
		got := util.GenBase58Upper(n)
		if len(got) != n {
			t.Errorf("GenBase58Upper(%d): got len=%d, want %d (value=%q)", n, len(got), n, got)
		}
	}
}

// TestGenBase58Upper_AlphabetOnly verifies output contains only base58-uppercase chars.
func TestGenBase58Upper_AlphabetOnly(t *testing.T) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	for i := 0; i < 500; i++ {
		code := util.GenBase58Upper(6)
		for _, ch := range code {
			if !strings.ContainsRune(alphabet, ch) {
				t.Errorf("GenBase58Upper(6): unexpected char %q in code %q", ch, code)
			}
		}
	}
}

// TestGenBase58Upper_ExcludesAmbiguous verifies 0, O, I never appear.
func TestGenBase58Upper_ExcludesAmbiguous(t *testing.T) {
	for i := 0; i < 1000; i++ {
		code := util.GenBase58Upper(6)
		for _, banned := range []rune{'0', 'O', 'I', 'l'} {
			if strings.ContainsRune(code, banned) {
				t.Errorf("GenBase58Upper(6): ambiguous char %q found in %q", banned, code)
			}
		}
	}
}

// TestGenBase58Upper_Uniqueness generates 1000 6-char codes and checks for collisions.
// 33^6 ≈ 1.3B possible values → collision at 1000 samples is astronomically unlikely;
// any collision would indicate a broken rand source.
func TestGenBase58Upper_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		code := util.GenBase58Upper(6)
		if _, dup := seen[code]; dup {
			t.Errorf("GenBase58Upper(6): duplicate code %q at iteration %d", code, i)
		}
		seen[code] = struct{}{}
	}
}

// TestGenBase58Upper_ZeroLen verifies empty string on n=0.
func TestGenBase58Upper_ZeroLen(t *testing.T) {
	got := util.GenBase58Upper(0)
	if got != "" {
		t.Errorf("GenBase58Upper(0): want empty string, got %q", got)
	}
}

// TestGenBase58Upper_UppercaseOnly verifies all chars are uppercase.
func TestGenBase58Upper_UppercaseOnly(t *testing.T) {
	for i := 0; i < 200; i++ {
		code := util.GenBase58Upper(8)
		if code != strings.ToUpper(code) {
			t.Errorf("GenBase58Upper(8): got lowercase in %q", code)
		}
	}
}
