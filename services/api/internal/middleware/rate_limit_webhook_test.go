// rate_limit_webhook_test.go — unit tests for NewRateLimitWebhook using miniredis.
package middleware_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func newTestApp(rdb *redis.Client, perSec int) *fiber.App {
	log, _ := zap.NewDevelopment()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.NewRateLimitWebhook(rdb, perSec, log))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestRateLimitWebhook_UnderLimit(t *testing.T) {
	_, rdb := newTestRedis(t)
	app := newTestApp(rdb, 20)

	// 20 requests — all should pass.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i, resp.StatusCode)
		}
	}
}

func TestRateLimitWebhook_ExceedsLimit_Returns429(t *testing.T) {
	_, rdb := newTestRedis(t)
	app := newTestApp(rdb, 20)

	// First 20 pass.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
	}
	// 21st request in same 1s window → 429.
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("21st request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("21st request: want 429, got %d", resp.StatusCode)
	}
}

func TestRateLimitWebhook_NewWindowAllows(t *testing.T) {
	mr, rdb := newTestRedis(t)
	app := newTestApp(rdb, 20)

	// Exhaust the window.
	for i := 0; i < 21; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req)
		resp.Body.Close()
	}

	// Fast-forward miniredis TTL by 2 seconds — key expires.
	mr.FastForward(2 * time.Second)

	// Now a fresh request should be allowed.
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("post-window request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("post-window request: want 200, got %d", resp.StatusCode)
	}
}

func TestRateLimitWebhook_NilRedis_PassThrough(t *testing.T) {
	// Nil Redis (dev mode) — middleware must pass through without panicking.
	log, _ := zap.NewDevelopment()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.NewRateLimitWebhook(nil, 20, log))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	for i := 0; i < 30; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("nil redis request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("nil redis request %d: want 200, got %d", i, resp.StatusCode)
		}
	}
}
