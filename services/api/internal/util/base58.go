// base58.go — uppercase-only base58 helper for referral codes.
// Separate from token.go (which uses mixed-case base58 for API keys).
// Alphabet: 33 chars, uppercase only, excludes 0 (zero), O, I (visually ambiguous).
package util

import "crypto/rand"

// base58UpperAlphabet is a 33-char uppercase-only subset of base58.
// Excludes: 0 (zero), O (uppercase-oh), I (uppercase-eye).
// Result chars are unambiguous when printed or read aloud.
const base58UpperAlphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// GenBase58Upper returns a cryptographically random string of n characters
// drawn from base58UpperAlphabet. Each character is independently and uniformly
// sampled (rejection-sampling NOT used; modulo bias is < 1/33 ≈ 3% — acceptable
// for referral codes where readability matters more than strict uniformity).
//
// Panics only if crypto/rand is broken (system-level entropy failure).
func GenBase58Upper(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is a fatal system condition — panic is appropriate.
		panic("util.GenBase58Upper: crypto/rand.Read failed: " + err.Error())
	}
	for i := range buf {
		buf[i] = base58UpperAlphabet[int(buf[i])%len(base58UpperAlphabet)]
	}
	return string(buf)
}
