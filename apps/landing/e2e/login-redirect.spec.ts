import { expect, type Page, test } from "@playwright/test";

async function submitApiKey(page: Page, key: string) {
  await expect(page.getByRole("button", { name: "Đăng nhập" })).toBeEnabled();
  const input = page.getByLabel("API key");
  await input.fill(key);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
}

const cases = [
  ["https://evil.test", "/dashboard"],
  ["/login", "/dashboard"],
  ["/api/proxy", "/dashboard"],
  ["/dashboard%2F..", "/dashboard"],
  ["/dashboard/../admin", "/dashboard"],
  ["javascript:alert(1)", "/dashboard"],
  ["/dashboard", "/dashboard"],
  ["/sites", "/sites"],
] as const;

test.describe("login next redirect sanitization", () => {
  for (const [next, expectedPath] of cases) {
    test(`next=${next} redirects to ${expectedPath}`, async ({ page }) => {
      await page.goto(`/login?next=${encodeURIComponent(next)}`);
      await submitApiKey(page, "sbf_live_valid_e2e_key");
      await expect(page).toHaveURL(new RegExp(`${expectedPath}$`), { timeout: 10_000 });
    });
  }
});
