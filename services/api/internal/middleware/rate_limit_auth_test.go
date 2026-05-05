package middleware_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newAuthVerifyRateLimitApp(rdb *redis.Client) *fiber.App {
	log, _ := zap.NewDevelopment()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.NewAuthVerifyRateLimit(rdb, log))
	app.Post("/auth/verify", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestAuthVerifyRateLimit_NilRedis_Returns503(t *testing.T) {
	app := newAuthVerifyRateLimitApp(nil)

	req := httptest.NewRequest("POST", "/auth/verify", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
}

func TestAuthVerifyRateLimit_ExceedsMinuteLimit_Returns429(t *testing.T) {
	_, rdb := newTestRedis(t)
	app := newAuthVerifyRateLimitApp(rdb)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/auth/verify", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i, resp.StatusCode)
		}
	}

	req := httptest.NewRequest("POST", "/auth/verify", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("6th request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("6th request: want 429, got %d", resp.StatusCode)
	}
}

func TestAuthVerifyRateLimit_NewMinuteAllows(t *testing.T) {
	mr, rdb := newTestRedis(t)
	app := newAuthVerifyRateLimitApp(rdb)

	for i := 0; i < 6; i++ {
		req := httptest.NewRequest("POST", "/auth/verify", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
	}

	mr.FastForward(61 * time.Second)

	req := httptest.NewRequest("POST", "/auth/verify", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("post-window request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("post-window request: want 200, got %d", resp.StatusCode)
	}
}
