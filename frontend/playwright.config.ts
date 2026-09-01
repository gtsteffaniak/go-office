import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";

const ciTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const workers = Number(process.env.PLAYWRIGHT_WORKERS ?? 10);

export default defineConfig({
  // Fail fast in CI: happy path ~30–60s/test; cap hung tests so retries finish within 10m job limit.
  timeout: Number(process.env.PLAYWRIGHT_TEST_TIMEOUT ?? (ciTest ? 85_000 : 90_000)),
  expect: {
    timeout: Number(process.env.PLAYWRIGHT_EXPECT_TIMEOUT ?? (ciTest ? 6_000 : 10_000)),
  },
  testDir: "./tests/playwright/office",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: ciTest ? 1 : 1,
  workers,
  reporter: "line",
  grep: process.env.PLAYWRIGHT_GREP ? new RegExp(process.env.PLAYWRIGHT_GREP) : undefined,
  use: {
    baseURL,
    actionTimeout: Number(process.env.PLAYWRIGHT_ACTION_TIMEOUT ?? (ciTest ? 12_000 : 15_000)),
    navigationTimeout: Number(process.env.PLAYWRIGHT_NAVIGATION_TIMEOUT ?? (ciTest ? 25_000 : 30_000)),
    trace: "on-first-retry",
    locale: "en-US",
  },
  projects: [
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },
  ],
});
