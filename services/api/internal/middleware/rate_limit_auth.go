package middleware

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type AuthVerifyLimits struct {
	PerMinute int
	PerHour   int
	PerDay    int
}

var defaultAuthVerifyLimits = AuthVerifyLimits{PerMinute: 5, PerHour: 20, PerDay: 200}

func rateLimitIP(c *fiber.Ctx) string {
	ip := ConnIPFromCtx(c)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	if ip != "" {
		return ip
	}
	return c.IP()
}

func NewAuthVerifyRateLimit(rdb *goredis.Client, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rdb == nil {
			if log != nil {
				log.Warn("auth verify rate-limit redis unavailable", zap.String("ip", rateLimitIP(c)))
			}
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate_limit_unavailable"})
		}

		ip := rateLimitIP(c)

		windows := []struct {
			unit  string
			limit int
			ttl   time.Duration
		}{
			{"min", defaultAuthVerifyLimits.PerMinute, time.Minute},
			{"hr", defaultAuthVerifyLimits.PerHour, time.Hour},
			{"day", defaultAuthVerifyLimits.PerDay, 24 * time.Hour},
		}

		for _, window := range windows {
			key := fmt.Sprintf("rl:auth_verify:%s:%s:%d", window.unit, ip, time.Now().Truncate(window.ttl).Unix())
			pipe := rdb.Pipeline()
			incr := pipe.Incr(context.Background(), key)
			pipe.Expire(context.Background(), key, window.ttl)
			if _, err := pipe.Exec(context.Background()); err != nil {
				if log != nil {
					log.Warn("auth verify rate-limit redis error", zap.String("ip", ip), zap.Error(err))
				}
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate_limit_unavailable"})
			}
			if int(incr.Val()) > window.limit {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too_many_attempts", "retry_after": int(window.ttl.Seconds())})
			}
		}

		return c.Next()
	}
}
