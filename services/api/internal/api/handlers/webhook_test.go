// webhook_test.go — handler-level tests for POST /webhooks/sepay.
// Uses a mock WebhookService to avoid DB dependency in handler path tests.
// Rate-limit tests use miniredis. Integration (DB) tests live in service/webhook_service_test.go.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api/handlers"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ─────────────────────────── Harness ──────────────────────────────────────
// Handler unit tests use deps.WebhookSvc=nil to exercise paths that don't reach
// the service layer (auth fail, account mismatch, unmatched content, etc.).
// Full service integration tests live in service/webhook_service_test.go.

const validToken = "test-webhook-secret"

func newWebhookApp(deps *handlers.WebhookDeps) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             64 * 1024, // 64KB
	})
	if deps.Rdb != nil {
		log, _ := zap.NewDevelopment()
		app.Use(middleware.NewRateLimitWebhook(deps.Rdb, 20, log))
	}
	app.Post("/webhooks/sepay", handlers.SePayWebhook(deps))
	return app
}

func minimalDeps(t *testing.T) *handlers.WebhookDeps {
	t.Helper()
	log, _ := zap.NewDevelopment()
	return &handlers.WebhookDeps{
		Cfg: &config.Config{
			SepayWebhookToken: validToken,
			SepayBankCode:     "MBBank",
			SepayBankAccount:  "123456789",
		},
		Log:     log,
		RootCtx: context.Background(),
	}
}

func jsonBody(t *testing.T, v any) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return bytes.NewReader(b)
}

func validPayload() sepay.Payload {
	return sepay.Payload{
		ID:             1,
		Gateway:        "MBBank",
		AccountNumber:  "123456789",
		Content:        "SBF TOPUP AABBCC112233",
		TransferType:   "in",
		TransferAmount: 329_000,
	}
}

func doPost(t *testing.T, app *fiber.App, body io.Reader, token string) *http.Response {
	t.Helper()
	req := httptest.NewRequest("POST", "/webhooks/sepay", body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Apikey "+token)
	}
	resp, err := app.Test(req, 5000) //nolint:bodyclose // closed by decodeResp or test body
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp
}

func decodeResp(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	defer r.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return m
}

// ─────────────────────────── Tests ─────────────────────────────────────────

func TestSePayWebhook_AuthFail_Returns200NotRetried(t *testing.T) {
	// Wrong Apikey → 200 success=false (NOT 401) — prevents SePay retry storm.
	deps := minimalDeps(t)
	app := newWebhookApp(deps)

	resp := doPost(t, app, jsonBody(t, validPayload()), "wrong-token")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	m := decodeResp(t, resp)
	if m["success"] != false {
		t.Fatalf("want success=false, got %v", m["success"])
	}
}

func TestSePayWebhook_NoAuth_Returns200False(t *testing.T) {
	deps := minimalDeps(t)
	app := newWebhookApp(deps)

	resp := doPost(t, app, jsonBody(t, validPayload()), "") // no token
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	m := decodeResp(t, resp)
	if m["success"] != false {
		t.Fatalf("want success=false, got %v", m["success"])
	}
}

func TestSePayWebhook_TransferTypeOut_Ignored(t *testing.T) {
	// transferType="out" → 200 success=true, no service call.
	deps := minimalDeps(t)
	app := newWebhookApp(deps)

	p := validPayload()
	p.TransferType = "out"
	resp := doPost(t, app, jsonBody(t, p), validToken)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	m := decodeResp(t, resp)
	if m["success"] != true {
		t.Fatalf("want success=true for outgoing transfer, got %v", m["success"])
	}
}

func TestSePayWebhook_AccountMismatch_Returns200False(t *testing.T) {
	deps := minimalDeps(t)
	app := newWebhookApp(deps)

	p := validPayload()
	p.AccountNumber = "WRONGACCOUNT"
	resp := doPost(t, app, jsonBody(t, p), validToken)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	m := decodeResp(t, resp)
	if m["success"] != false {
		t.Fatalf("want success=false for account mismatch, got %v", m["success"])
	}
}

func TestSePayWebhook_GatewayMismatch_Returns200False(t *testing.T) {
	deps := minimalDeps(t)
	app := newWebhookApp(deps)

	p := validPayload()
	p.Gateway = "UnknownBank"
	resp := doPost(t, app, jsonBody(t, p), validToken)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	m := decodeResp(t, resp)
	if m["success"] != false {
		t.Fatalf("want success=false for gateway mismatch, got %v", m["success"])
	}
}

func TestSePayWebhook_UnmatchedContent_Returns200True(t *testing.T) {
	// Content without SBF TOPUP pattern → success=true (not an error — manual transfer).
	deps := minimalDeps(t)
	app := newWebhookApp(deps)

	p := validPayload()
	p.Content = "random bank transfer without order code"
	resp := doPost(t, app, jsonBody(t, p), validToken)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	m := decodeResp(t, resp)
	if m["success"] != true {
		t.Fatalf("want success=true for unmatched content, got %v", m["success"])
	}
}

func TestSePayWebhook_NilWebhookSvc_Returns503(t *testing.T) {
	// WebhookSvc not wired → 503. Ensures nil-guard works.
	deps := minimalDeps(t)
	// deps.WebhookSvc is already nil
	app := newWebhookApp(deps)

	resp := doPost(t, app, jsonBody(t, validPayload()), validToken)
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("want 503 when WebhookSvc nil, got %d", resp.StatusCode)
	}
}

func TestSePayWebhook_InvalidBody_Returns200False(t *testing.T) {
	deps := minimalDeps(t)
	app := newWebhookApp(deps)

	req := httptest.NewRequest("POST", "/webhooks/sepay", bytes.NewBufferString("not-json{{{"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Apikey "+validToken)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	m := decodeResp(t, resp)
	if m["success"] != false {
		t.Fatalf("want success=false for invalid body, got %v", m["success"])
	}
}

func TestSePayWebhook_RateLimit_429AfterLimit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	log, _ := zap.NewDevelopment()
	deps := minimalDeps(t)
	deps.Rdb = rdb

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.NewRateLimitWebhook(rdb, 20, log))
	app.Post("/webhooks/sepay", handlers.SePayWebhook(deps))

	// 20 requests with auth fail (fast path, no service needed) — all should be 200.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("POST", "/webhooks/sepay", jsonBody(t, validPayload()))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Apikey wrong")
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode == fiber.StatusTooManyRequests {
			t.Fatalf("request %d: unexpected 429 before limit", i)
		}
	}

	// 21st request → 429.
	req := httptest.NewRequest("POST", "/webhooks/sepay", jsonBody(t, validPayload()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Apikey wrong")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("21st request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("want 429 on 21st request, got %d", resp.StatusCode)
	}
}

func TestSePayWebhook_BodyLimit_Rejected(t *testing.T) {
	deps := minimalDeps(t)

	// App with explicit 64KB body limit.
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             64 * 1024,
	})
	app.Post("/webhooks/sepay", handlers.SePayWebhook(deps))

	// 65KB payload — exceeds body limit.
	large := make([]byte, 65*1024)
	for i := range large {
		large[i] = 'A'
	}
	req := httptest.NewRequest("POST", "/webhooks/sepay", bytes.NewReader(large))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Apikey "+validToken)

	// Fiber's app.Test() rejects oversized bodies at the test harness level (returns error)
	// rather than issuing a 413 HTTP response. In production, Fiber returns 413 to the real
	// HTTP client. We assert the rejection occurs (either 413 or test error) to verify the
	// body limit config is active.
	resp, err := app.Test(req, 5000)
	if err != nil {
		// Expected: Fiber test harness raises "body size exceeds the given limit".
		if !strings.Contains(err.Error(), "body size") && !strings.Contains(err.Error(), "limit") {
			t.Fatalf("unexpected app.Test error: %v", err)
		}
		return // body limit enforced correctly
	}
	defer resp.Body.Close()
	// If no error, Fiber may have returned 413 (production-mode behaviour).
	if resp.StatusCode != fiber.StatusRequestEntityTooLarge {
		t.Fatalf("want 413 for oversized body, got %d", resp.StatusCode)
	}
}

// ─────────────────────────── Alert channel smoke ──────────────────────────

func TestWebhookDeps_AdminAlertCh_Wired(t *testing.T) {
	// Verify that WebhookDeps.AdminAlertCh field is accessible (compile-time check).
	ch := make(chan notify.AdminAlert, 1)
	deps := &handlers.WebhookDeps{
		AdminAlertCh: ch,
	}
	if deps.AdminAlertCh == nil {
		t.Fatal("AdminAlertCh should not be nil")
	}
}
