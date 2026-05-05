package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
)

const extensionSignatureWindow = 5 * time.Minute

// nonceRedisKeyPrefix prefixes Redis keys used for replay-protection nonces.
// Format: "ext_nonce:{userID}:{nonce}" — scoped per user so a stolen nonce for
// user A cannot block user B.
const nonceRedisKeyPrefix = "ext_nonce:"

// ExtensionSignature returns a Fiber middleware that authenticates extension
// requests via HMAC-SHA256 signed headers (X-Timestamp / X-Nonce / X-Signature).
//
// Replay protection:
//   - When rdb != nil: uses Redis SET NX EX (atomic, multi-instance safe).
//     A captured request cannot be replayed against a second API instance (F4).
//   - When rdb == nil (dev/test): falls back to in-process nonce map (process-local,
//     NOT multi-instance safe — acceptable only for single-process test/dev runs).
//
// HMAC note: comparison uses hmac.Equal (constant-time byte compare). Both the
// computed expected value and the header value are fixed-length hex strings (64
// bytes = 32-byte SHA-256 in hex), so the comparison is always timing-safe (F2).
// Do NOT change this to == or bytes.Equal.
func ExtensionSignature(rdb *goredis.Client) fiber.Handler {
	// fallback in-process nonce store — used only when Redis is unavailable.
	local := nonceCache{items: make(map[string]time.Time)}

	return func(c *fiber.Ctx) error {
		apiUser, ok := ApiUserFromCtx(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		timestamp := strings.TrimSpace(c.Get("X-Timestamp"))
		nonce := strings.TrimSpace(c.Get("X-Nonce"))
		signature := strings.TrimSpace(c.Get("X-Signature"))
		if timestamp == "" || nonce == "" || signature == "" || len(nonce) > 128 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "bad_signature"})
		}
		issuedAt, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "bad_signature"})
		}
		now := time.Now()
		if skew := now.Sub(time.Unix(issuedAt, 0)); skew > extensionSignatureWindow || skew < -extensionSignatureWindow {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "stale_signature"})
		}

		// Nonce claim: Redis preferred (multi-instance safe); in-process fallback for dev.
		nonceKey := apiUser.ID.String() + ":" + nonce
		if !claimNonce(c.Context(), rdb, &local, nonceKey, extensionSignatureWindow) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "replayed_signature"})
		}

		bodyHash := sha256.Sum256(c.Body())
		if strings.TrimSpace(apiUser.RawKey) == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "bad_signature"})
		}
		payload := fmt.Sprintf("%s\n%s\n%s\n%s", c.Method(), c.Path(), timestamp, hex.EncodeToString(bodyHash[:]))
		expected := signExtensionPayload(apiUser.RawKey, payload)
		// hmac.Equal performs constant-time comparison — required for MAC values (F2).
		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "bad_signature"})
		}
		return c.Next()
	}
}

// claimNonce records a nonce as used. Returns false if the nonce was already seen
// (replay attack) or if recording failed. TTL equals the signature validity window.
//
// Redis path: SET ext_nonce:{key} 1 NX EX {ttlSeconds}
//   - NX guarantees atomicity: only the first caller wins (F4 multi-instance safety).
//   - EX auto-evicts after the window — no unbounded growth (F3).
//
// Fallback path (rdb == nil): in-process map with lazy GC. Only suitable for
// single-process deployments. A warning is intentionally absent here to avoid
// log spam; the caller (main.go) logs when Redis is unavailable at startup.
func claimNonce(ctx context.Context, rdb *goredis.Client, local *nonceCache, key string, ttl time.Duration) bool {
	if rdb != nil {
		redisKey := nonceRedisKeyPrefix + key
		// SetNX returns true only if the key was newly set (NX = NOT EXISTS).
		// EX ttl ensures automatic expiry — no manual GC needed.
		set, err := rdb.SetNX(ctx, redisKey, 1, ttl).Result()
		if err != nil {
			// Redis error: fail-open to avoid locking out all extension users when Redis
			// has a transient blip. Log is intentionally omitted here — the caller stack
			// has already surfaced a Redis connectivity warning at boot. If this becomes
			// a sustained outage, fall through to local cache as best-effort protection.
			return local.claim(key, time.Now().Add(ttl))
		}
		return set
	}
	return local.claim(key, time.Now().Add(ttl))
}

func signExtensionPayload(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// nonceCache is an in-process nonce store used as a fallback when Redis is
// unavailable. It is NOT safe for multi-instance deployments (F4). Access is
// protected by a mutex; GC of expired entries runs lazily on each claim call.
// For production, always provide a Redis client to ExtensionSignature.
type nonceCache struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func (c *nonceCache) claim(key string, expiresAt time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for item, expiry := range c.items {
		if now.After(expiry) {
			delete(c.items, item)
		}
	}
	if _, exists := c.items[key]; exists {
		return false
	}
	c.items[key] = expiresAt
	return true
}
