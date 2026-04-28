import { expect, test } from "@playwright/test";
import { seedAuthCookie } from "./fixtures/auth";

test("proxy rejects request bodies above 1MB", async ({ page, request }) => {
  await seedAuthCookie(page);
  const cookies = await page.context().cookies();
  const cookieHeader = cookies.map((cookie) => `${cookie.name}=${cookie.value}`).join("; ");
  const response = await request.post("/api/proxy/api/v1/echo", {
    data: "x".repeat(2 * 1024 * 1024),
    headers: {
      "content-type": "text/plain",
      cookie: cookieHeader,
    },
  });

  expect(response.status()).toBe(413);
  expect(await response.json()).toEqual({ error: "payload_too_large", max_bytes: 1024 * 1024 });
});

test("proxy rejects oversized bodies without content-length", async ({ page }) => {
  await seedAuthCookie(page);
  await page.goto("/");

  const response = await page.evaluate(async () => {
    const res = await fetch("/api/proxy/api/v1/echo", {
      method: "POST",
      body: "x".repeat(2 * 1024 * 1024),
      headers: { "content-type": "text/plain" },
    });
    return { status: res.status, body: await res.json() };
  });

  expect(response).toEqual({
    status: 413,
    body: { error: "payload_too_large", max_bytes: 1024 * 1024 },
  });
});
