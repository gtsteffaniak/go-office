import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";
const isCI = !!process.env.CI;
const workers = 10;
export default defineConfig({
  // Hard cap per test attempt — hung document opens should fail well under this.
  timeout: 30000,
  expect: {
    timeout: 5000,
  },
  testDir: "./tests/playwright/office",
  fullyParallel: true,
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  workers,
  reporter: isCI ? "line" : "list",
  use: {
    baseURL,
    actionTimeout: 5000,
    navigationTimeout: 15000,
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
