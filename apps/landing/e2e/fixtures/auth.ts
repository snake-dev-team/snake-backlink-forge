import type { Page } from "@playwright/test";

const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const cookieName = baseURL.startsWith("http://") ? "sbf_key" : "__Host-sbf_key";
const secureCookie = baseURL.startsWith("https://");

export async function seedAuthCookie(page: Page, key = "sbf_live_e2e_test_key") {
  await page.context().addCookies([
    {
      name: cookieName,
      value: key,
      url: baseURL,
      sameSite: "Strict",
      httpOnly: true,
      secure: secureCookie,
    },
  ]);
}

export async function mockAppApi(page: Page) {
  await page.route("**/api/proxy/api/v1/me", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      status: 200,
      body: JSON.stringify({
        user_id: "00000000-0000-0000-0000-000000000001",
        telegram_id: 990000000001,
        telegram_username: "phase8_e2e",
        language: "vi",
        is_verified: true,
        balance_credits: 42,
        premium_credits: 7,
        standard_credits: 35,
        key_prefix: "sbf_live_pha",
      }),
    });
  });

  await page.route("**/api/proxy/api/v1/transactions?**", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      status: 200,
      body: JSON.stringify({ items: [], limit: 5, offset: 0 }),
    });
  });

  await page.route("**/api/proxy/api/v1/wp-sites", async (route) => {
    const request = route.request();
    if (request.method() === "POST") {
      await route.fulfill({
        contentType: "application/json",
        status: 201,
        body: JSON.stringify({
          site: {
            id: "00000000-0000-0000-0000-000000000002",
            base_url: "https://example.com",
            app_username: "admin",
            label: "E2E site",
            status: "connected",
            last_validated_at: new Date("2026-04-28T00:00:00Z").toISOString(),
            last_error: null,
            created_at: new Date("2026-04-28T00:00:00Z").toISOString(),
            updated_at: new Date("2026-04-28T00:00:00Z").toISOString(),
          },
        }),
      });
      return;
    }

    await route.fulfill({
      contentType: "application/json",
      status: 200,
      body: JSON.stringify({ items: [] }),
    });
  });
}
