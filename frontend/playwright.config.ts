import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";

// Set in Dockerfile.playwright-* — x2t runs one conversion at a time, so Docker
// tests must not hammer the server with parallel document opens.
const bundledTest = process.env.GO_OFFICE_PLAYWRIGHT_TEST === "true";

export default defineConfig({
  timeout: bundledTest ? 120_000 : 90_000,
  expect: {
    timeout: bundledTest ? 15_000 : 10_000,
  },
  testDir: "./tests/playwright/office",
  fullyParallel: !bundledTest,
  forbidOnly: !!process.env.CI,
  retries: bundledTest ? 2 : 1,
  workers: Number(process.env.PLAYWRIGHT_WORKERS ?? (bundledTest ? 1 : 4)),
  reporter: "line",
  use: {
    baseURL,
    actionTimeout: bundledTest ? 30_000 : 15_000,
    navigationTimeout: bundledTest ? 30_000 : 15_000,
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
