import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";
const workers = Number(process.env.PLAYWRIGHT_WORKERS ?? 8);
// Save-project budget. A healthy cell/word save test completes in roughly 60-90s (the
// server-side flush is ~6s; the rest is editor boot and readiness polling). 240s allowed
// ~3x slack, so a hang burned the full budget and, with 8 parallel workers draining, a
// failing project took ~400s to report. 150s keeps headroom over the slowest observed
// pass while halving the cost of a hang.
const saveTestTimeout = Number(process.env.PLAYWRIGHT_SAVE_TEST_TIMEOUT ?? 150_000);

const sharedUse = {
  baseURL,
  actionTimeout: Number(process.env.PLAYWRIGHT_ACTION_TIMEOUT ?? 45_000),
  navigationTimeout: Number(process.env.PLAYWRIGHT_NAVIGATION_TIMEOUT ?? 25_000),
  trace: (process.env.PLAYWRIGHT_TRACE as "on" | "off" | "retain-on-failure" | "on-first-retry") ??
    "retain-on-failure",
  locale: "en-US",
};

export default defineConfig({
  timeout: Number(process.env.PLAYWRIGHT_TEST_TIMEOUT ?? 120_000),
  expect: {
    timeout: Number(process.env.PLAYWRIGHT_EXPECT_TIMEOUT ?? 6_000),
  },
  testDir: "./tests/playwright/office",
  globalSetup: "./tests/playwright/global-setup.ts",
  globalTeardown: "./tests/playwright/global-teardown.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers,
  reporter: process.env.CI ? [["list"], ["line"]] : "line",
  grep: process.env.PLAYWRIGHT_GREP ? new RegExp(process.env.PLAYWRIGHT_GREP) : undefined,
  use: sharedUse,
  projects: [
    {
      name: "chromium",
      testIgnore: /(save|rtf-formatting|post-save-stability)\.spec\.ts$/,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "chromium-save",
      testMatch: /(save|rtf-formatting)\.spec\.ts$/,
      dependencies: ["chromium"],
      timeout: saveTestTimeout,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "chromium-post-save",
      testMatch: /post-save-stability\.spec\.ts$/,
      dependencies: ["chromium"],
      timeout: saveTestTimeout,
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
