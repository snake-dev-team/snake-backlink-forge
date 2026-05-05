import { expect, test } from "@playwright/test";
import { mockAppApi, seedAuthCookie } from "./fixtures/auth";

test("authed dashboard renders mocked account data", async ({ page }) => {
  await mockAppApi(page);
  await seedAuthCookie(page);

  await page.goto("/dashboard");

  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  await expect(page.getByText("42")).toBeVisible();
  await expect(page.getByText("Thao tác nhanh")).toBeVisible();
});
