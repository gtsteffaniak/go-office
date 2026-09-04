import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";

const ciTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const workers = Number(process.env.PLAYWRIGHT_WORKERS ?? (ciTest ? 4 : 10));

const sharedUse = {
  baseURL,
  actionTimeout: Number(process.env.PLAYWRIGHT_ACTION_TIMEOUT ?? (ciTest ? 12_000 : 15_000)),
  navigationTimeout: Number(process.env.PLAYWRIGHT_NAVIGATION_TIMEOUT ?? (ciTest ? 25_000 : 30_000)),
  trace: "on-first-retry" as const,
  locale: "en-US",
};

export default defineConfig({
  // Save tests: editor ready (25s) + interactive (90s) + save poll (90s) after open/content suite.
  timeout: Number(process.env.PLAYWRIGHT_TEST_TIMEOUT ?? (ciTest ? 180_000 : 90_000)),
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
      workers: 1,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "chromium-post-save",
      testMatch: /post-save-stability\.spec\.ts$/,
      dependencies: ["chromium"],
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
