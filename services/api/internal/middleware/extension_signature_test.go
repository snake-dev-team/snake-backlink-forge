package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// buildExtensionApp constructs a test Fiber app that sets up the ApiUser local
// and applies the ExtensionSignature middleware (nil Redis → in-process nonce cache).
func buildExtensionApp(rawKey string) *fiber.App {
	app := fiber.New()
	app.Get("/campaign/next", func(c *fiber.Ctx) error {
		c.Locals(ctxKeyApiUser, ApiUser{ID: uuid.New(), KeyPrefix: "sbf_live_abc", RawKey: rawKey})
		return ExtensionSignature(nil /* nil → in-process fallback, OK for tests */)(c)
	}, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	return app
}

func makeExtensionRequest(nonce, key string) *http.Request {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := sha256.Sum256(nil)
	payload := fmt.Sprintf("GET\n/campaign/next\n%s\n%s", timestamp, hex.EncodeToString(bodyHash[:]))
	req := httptest.NewRequest(fiber.MethodGet, "/campaign/next", nil)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signExtensionPayload(key, payload))
	return req
}

func TestExtensionSignatureUsesRawKeySecret(t *testing.T) {
	app := buildExtensionApp("sbf_live_secret_value")

	resp, err := app.Test(makeExtensionRequest("nonce-raw-key", "sbf_live_secret_value"))
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected status %d, got %d", fiber.StatusNoContent, resp.StatusCode)
	}
}

func TestExtensionSignatureRejectsPrefixBasedSignature(t *testing.T) {
	app := buildExtensionApp("sbf_live_secret_value")

	// Sign with the key prefix instead of the raw key — must be rejected.
	resp, err := app.Test(makeExtensionRequest("nonce-prefix", "sbf_live_abc"))
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
	}
}
