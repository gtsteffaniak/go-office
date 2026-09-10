import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";
const workers = Number(process.env.PLAYWRIGHT_WORKERS ?? 10);
const saveWorkers = Number(process.env.PLAYWRIGHT_SAVE_WORKERS ?? 4);
const saveTestTimeout = Number(process.env.PLAYWRIGHT_SAVE_TEST_TIMEOUT ?? 240_000);

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
      testIgnore: /(save|rtf-formatting|post-save-stability|content-ready)\.spec\.ts$/,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "chromium-save",
      testMatch: /(save|rtf-formatting|content-ready)\.spec\.ts$/,
      dependencies: ["chromium"],
      timeout: saveTestTimeout,
      workers: saveWorkers,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "chromium-post-save",
      testMatch: /post-save-stability\.spec\.ts$/,
      dependencies: ["chromium"],
      timeout: saveTestTimeout,
      workers: saveWorkers,
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
