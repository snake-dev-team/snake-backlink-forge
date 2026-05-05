import { defineConfig, devices } from "@playwright/test";

const appDir = __dirname;

export default defineConfig({
  testDir: "./e2e",
  timeout: 45_000,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  use: {
    baseURL: process.env.E2E_BASE_URL || "http://127.0.0.1:3000",
    trace: "on-first-retry",
  },
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : [
        {
          command: "node e2e/mock-backend.mjs",
          cwd: appDir,
          url: "http://127.0.0.1:3101/api/v1/me",
          reuseExistingServer: false,
          timeout: 10_000,
        },
        {
          command: "node node_modules/next/dist/bin/next dev -H 127.0.0.1",
          cwd: appDir,
          url: "http://127.0.0.1:3000",
          reuseExistingServer: false,
          timeout: 120_000,
          env: {
            E2E_AUTH_MOCK: "1",
            NEXT_PUBLIC_API_BASE_URL: "http://127.0.0.1:3101",
            NEXT_PUBLIC_APP_URL: "http://localhost:3000",
          },
        },
      ],
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
