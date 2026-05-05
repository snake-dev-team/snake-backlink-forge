// redact.go — Phase 06 [C2]: PII-safe hashing helpers for audit_log metadata.
//
// Spec §security line 590-591 and §architecture line 91 mandate that raw IPs and account
// numbers MUST NOT be stored in audit_log. Use these helpers at every audit-log call site
// in the webhook flow.
package sepay

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashIP returns the full 32-char (64 hex chars) SHA-256 hash of an IP address.
// Storing the full hash avoids birthday-collision merging distinct IPs into one rate-limit
// key while remaining irreversible (spec §security line 590: "full 32-char hex").
func HashIP(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}

// HashAccountPrefix returns the first 12 hex chars of the SHA-256 hash of an account number.
// 12 chars = 48 bits of entropy — sufficient for forensic correlation; irreversible to the
// raw account number. Spec §architecture line 91: "received_acc_sha256_prefix: sha256Hex[:12]".
func HashAccountPrefix(acc string) string {
	sum := sha256.Sum256([]byte(acc))
	return hex.EncodeToString(sum[:])[:12]
}
