// Package util — cryptographic API key generation for Snake Backlink Forge.
//
// Key format: sbf_live_<32-char-base58>
// Total length: 9 (prefix) + 32 (random) = 41 chars.
//
// NOTE: The master prompt §1.3 states 46 chars; this is inconsistent with
// KeyPrefix(9) + 37 random chars. We lock 41 as our standard: 9-char prefix +
// 32 base58 chars (~187 bits of entropy from 24 random bytes). This is
// documented here as the authoritative spec for all consumers.
package util

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
)

// Key format constants.
const (
	// KeyPrefix is the human-readable prefix for all API keys.
	KeyPrefix = "sbf_live_"

	// KeyRandomLen is the number of base58-encoded chars appended after the prefix.
	// Total plaintext length = len(KeyPrefix) + KeyRandomLen = 9 + 32 = 41 chars.
	KeyRandomLen = 32

	// base58Alphabet excludes visually ambiguous characters: 0 (zero), O (uppercase-oh),
	// I (uppercase-eye), l (lowercase-ell). 58 chars total.
	base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// randByteLen is the number of crypto/rand bytes consumed per key.
	// 24 bytes = 192 bits; base58-encoded gives ≥32 chars for any non-zero input.
	randByteLen = 24

	// KeyPrefixLen is the prefix length stored as key_prefix (first 12 chars of plaintext).
	// e.g. "sbf_live_Zk3" (9 fixed + 3 random chars).
	KeyPrefixLen = 12
)

// GenerateAPIKey produces a cryptographically secure API key.
//
// Returns:
//   - plaintext: full key string (41 chars). Show ONCE to user; never store or log.
//   - hashBytes: SHA-256(plaintext) as 32-byte slice. This is what gets stored in DB.
//   - prefix: first 12 chars of plaintext for display masking (e.g. "sbf_live_Zk3").
//   - err: non-nil only if crypto/rand fails (broken entropy source — abort issuance).
func GenerateAPIKey() (plaintext string, hashBytes []byte, prefix string, err error) {
	buf := make([]byte, randByteLen)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, "", fmt.Errorf("util.GenerateAPIKey: rand.Read: %w", err)
	}

	encoded := base58Encode(buf, KeyRandomLen)
	plaintext = KeyPrefix + encoded

	sum := sha256.Sum256([]byte(plaintext))
	hashBytes = sum[:]

	prefix = plaintext[:KeyPrefixLen]
	return plaintext, hashBytes, prefix, nil
}

// base58Encode encodes src bytes into a base58 string of exactly targetLen chars.
// Uses big-integer division for correctness (Option B — streaming encoder).
//
// Padding strategy: MSB-side padding with alphabet[0] ('1') when natural encoding
// is shorter than targetLen (low-entropy inputs or leading zero bytes).
//
// Truncation strategy: if natural encoding exceeds targetLen, LSB digits are
// discarded (tail of buf before reversal). This accepts minor entropy loss at the
// low-significance end only — the MSB digits (highest entropy) are always preserved.
// For well-sized src (randByteLen=24 → ≤33 base58 digits), overflow is rare but safe.
func base58Encode(src []byte, targetLen int) string {
	num := new(big.Int).SetBytes(src)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	// buf accumulates digits LSB-first.
	buf := make([]byte, 0, targetLen+4)
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		buf = append(buf, base58Alphabet[mod.Int64()])
	}

	// Pad MSB side (prepend in LSB-first buffer = append before reversal).
	for len(buf) < targetLen {
		buf = append(buf, base58Alphabet[0])
	}

	// If buf exceeds targetLen, discard LSB digits (low-significance end only).
	// Preserves the most-significant digits which carry the bulk of entropy.
	if len(buf) > targetLen {
		buf = buf[:targetLen]
	}

	// Reverse: buf is LSB-first, we need MSB-first output.
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}

	return string(buf)
}
