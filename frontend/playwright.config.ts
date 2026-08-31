import { defineConfig, devices } from "@playwright/test";
import os from "node:os";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";

const ciTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";

function defaultWorkers(): number {
  if (process.env.PLAYWRIGHT_WORKERS) {
    return Number(process.env.PLAYWRIGHT_WORKERS);
  }
  if (ciTest) {
    return 6;
  }
  return Math.min(6, Math.max(2, os.cpus().length));
}

const workers = defaultWorkers();

export default defineConfig({
  timeout: ciTest ? 120_000 : 90_000,
  expect: {
    timeout: ciTest ? 15_000 : 10_000,
  },
  testDir: "./tests/playwright/office",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 1,
  workers,
  reporter: "line",
  grep: process.env.PLAYWRIGHT_GREP ? new RegExp(process.env.PLAYWRIGHT_GREP) : undefined,
  use: {
    baseURL,
    actionTimeout: ciTest ? 30_000 : 15_000,
    navigationTimeout: ciTest ? 60_000 : 30_000,
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
