import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";

export default defineConfig({
  // Hard cap per test attempt — hung document opens should fail well under this.
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  testDir: "./tests/playwright/office",
  fullyParallel: true,
  retries: 1,
  workers: 10,
  reporter: "line",
  use: {
    baseURL,
    actionTimeout: 5_000,
    navigationTimeout: 15_000,
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
