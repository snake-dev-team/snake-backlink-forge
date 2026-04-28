import { expect, type Page, test } from "@playwright/test";

async function submitApiKey(page: Page, key: string) {
  await expect(page.getByRole("button", { name: "Đăng nhập" })).toBeEnabled();
  const input = page.getByLabel("API key");
  await input.fill(key);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
}

test("shows bad-key error then logs in with a valid key", async ({ page }) => {
  await page.goto("/login");

  await submitApiKey(page, "sbf_live_bad_e2e_key");
  await expect(page.getByText("Khóa không hợp lệ — kiểm tra lại")).toBeVisible({ timeout: 10_000 });

  await submitApiKey(page, "sbf_live_valid_e2e_key");
  await expect(page).toHaveURL(/\/dashboard$/);
});
