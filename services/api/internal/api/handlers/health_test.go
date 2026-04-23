// Package handlers — smoke tests for health endpoints (C5 success criterion).
package handlers_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api/handlers"
)

// newTestApp builds a minimal Fiber app with the two health routes.
func newTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		// Disable startup banner in test output.
		DisableStartupMessage: true,
	})
	app.Get("/health", handlers.Health)
	app.Get("/ready", handlers.Ready(nil, nil)) // nil pool+rdb — Phase 1 lenient
	return app
}

// TestHealth_Returns200WithOKStatus verifies the liveness probe contract.
func TestHealth_Returns200WithOKStatus(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("want status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Error("Content-Type header missing")
	}

	body, _ := io.ReadAll(resp.Body)
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v — body: %s", err, body)
	}
	if payload["status"] != "ok" {
		t.Errorf("want status=ok, got %q — body: %s", payload["status"], body)
	}
}

// TestReady_NilDeps_Returns503 verifies lenient Phase-1 boot: nil pool+rdb must
// return 503 with both db and redis set to "err" rather than panicking.
func TestReady_NilDeps_Returns503(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Errorf("want status 503, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v — body: %s", err, body)
	}
	if payload["db"] != "err" {
		t.Errorf("want db=err, got %q", payload["db"])
	}
	if payload["redis"] != "err" {
		t.Errorf("want redis=err, got %q", payload["redis"])
	}
}

// TestHealth_ContentType verifies the response carries a JSON content-type.
func TestHealth_ContentType(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Fatal("Content-Type header is empty")
	}
	// Fiber sets "application/json" (may include charset).
	if len(ct) < len("application/json") {
		t.Errorf("unexpected Content-Type %q", ct)
	}
}
