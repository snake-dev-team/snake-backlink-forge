import { expect, test } from "@playwright/test";
import { mockAppApi, seedAuthCookie } from "./fixtures/auth";

test("connects a WordPress site through mocked proxy", async ({ page }) => {
  await mockAppApi(page);
  await seedAuthCookie(page);

  await page.goto("/sites/connect");
  await page.getByLabel("Site URL").fill("https://example.com");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Application Password").fill("xxxx xxxx xxxx xxxx xxxx xxxx");
  await page.getByLabel("Label").fill("E2E site");
  await page.getByRole("button", { name: "Connect WordPress" }).click();

  await expect(page).toHaveURL(/\/sites$/);
});

test("shows invalid credentials error from mocked proxy", async ({ page }) => {
  await mockAppApi(page);
  await seedAuthCookie(page);

  await page.goto("/sites/connect");
  await page.getByLabel("Site URL").fill("https://example.com");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Application Password").fill("wrong-password");
  await page.getByRole("button", { name: "Connect WordPress" }).click();

  await expect(page.getByText("Username hoặc Application Password không đúng.")).toBeVisible();
});

test("shows temporary server error from mocked proxy", async ({ page }) => {
  await mockAppApi(page);
  await seedAuthCookie(page);

  await page.goto("/sites/connect");
  await page.getByLabel("Site URL").fill("https://server-error.example.com");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Application Password").fill("xxxx xxxx xxxx xxxx xxxx xxxx");
  await page.getByRole("button", { name: "Connect WordPress" }).click();

  await expect(page.getByText("Site server tạm thời không phản hồi, thử lại sau.")).toBeVisible();
});
