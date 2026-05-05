// verify.go — Phase 06: constant-time Apikey verification for SePay webhook auth.
package sepay

import (
	"crypto/subtle"
	"strings"
)

// VerifyApikey extracts the token from an "Apikey <token>" Authorization header value
// and performs a constant-time comparison against the expected token.
//
// Returns false when:
//   - expected is empty (prevents misconfig auto-allow — fail-closed)
//   - header is empty or does not start with "Apikey "
//   - token does not match expected
//
// crypto/subtle.ConstantTimeCompare prevents timing-side-channel attacks where an
// attacker could deduce token length or prefix by measuring response time differences.
func VerifyApikey(authHeader, expected string) bool {
	if expected == "" {
		return false
	}
	if !strings.HasPrefix(authHeader, "Apikey ") {
		return false
	}
	token := strings.TrimPrefix(authHeader, "Apikey ")
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}
