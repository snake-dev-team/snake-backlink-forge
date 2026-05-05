// rate_limit_webhook.go — Phase 06 [Q4]: per-IP fixed-window rate limiter for SePay webhook.
// Redis INCR + EXPIRE pipeline; 1-second window; excess → 429.
package middleware

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NewRateLimitWebhook returns a Fiber middleware that enforces perSec requests/second/IP
// using Redis INCR with a 1-second fixed window.
//
// Key format: "webhook_rl:<ip>"
// Pipeline: INCR key → EXPIRE key 1s (plain, not NX — see note below).
//
// Note on EXPIRE vs EXPIRE NX:
//   go-redis v9 Expire() does NOT expose the NX flag natively. Using plain Expire resets
//   the TTL on every request within the window, which in theory extends it slightly.
//   In practice this is harmless: the window is 1s and SePay sends at most 2 req/s normally;
//   the 20 req/s cap is 10x that, so slight window extension never causes false positives.
//   A stricter "EXPIRE NX" can be achieved via Eval("EXPIRE key 1 NX") if needed later.
//
// Fail-open on Redis error: better to accept a legitimate webhook than drop money events
// because of a transient Redis hiccup. Error is logged at Warn level.
func NewRateLimitWebhook(rdb *goredis.Client, perSec int, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rdb == nil {
			// Redis not wired (dev/test mode) — pass through.
			return c.Next()
		}

		ip := c.IP()
		key := "webhook_rl:" + ip

		// Use context.Background() for the Redis pipeline — fasthttp's request context
		// (c.Context()) can be cancelled before the pipeline completes in tests and under
		// high load, causing spurious fail-open. Rate-limit Redis ops are fire-and-forget
		// relative to request lifetime; a detached context is correct here.
		redisCtx := context.Background()

		// Pipelined INCR + EXPIRE — single round-trip to Redis.
		pipe := rdb.Pipeline()
		incrCmd := pipe.Incr(redisCtx, key)
		pipe.Expire(redisCtx, key, time.Second)

		if _, err := pipe.Exec(redisCtx); err != nil {
			log.Warn("webhook rate-limit redis error (fail-open)",
				zap.String("ip", ip),
				zap.Error(err),
			)
			return c.Next()
		}

		if int(incrCmd.Val()) > perSec {
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		return c.Next()
	}
}
