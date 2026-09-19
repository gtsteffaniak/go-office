import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080";
const workers = Number(process.env.PLAYWRIGHT_WORKERS ?? 8);
// Save-project budget. A healthy cell/word save test completes in roughly 60-90s (the
// server-side flush is ~6s; the rest is editor boot and readiness polling). 240s allowed
// ~3x slack, so a hang burned the full budget and, with 8 parallel workers draining, a
// failing project took ~400s to report. 150s keeps headroom over the slowest observed
// pass while halving the cost of a hang.
//
// The save project carves an explicit teardown reserve out of its own budget (see the gate
// sizes in tests/playwright/editor.ts), so the in-test gates always finish first and any
// failure is reported on its own terms. The default project timeout stays separate: setting
// both to the same 150s is what made an expiry indistinguishable from a genuine hang, since
// the test died in teardown before it could report why it was waiting.
const saveTestTimeout = Number(process.env.PLAYWRIGHT_SAVE_TEST_TIMEOUT ?? 150_000);
const defaultTestTimeout = Number(process.env.PLAYWRIGHT_TEST_TIMEOUT ?? 120_000);
// The rtf-formatting specs run TWO full edit→save→verify cycles inside one test (the marker
// is persisted first, then formatting is applied to it and that must persist too), so their
// worst case is roughly double a plain save test. Sharing the 150s save budget meant the
// second cycle could never finish on a loaded worker, and every one of these tests failed
// with a bare "Test timeout ... exceeded" at the final format-verify poll. Give the project
// that actually does two cycles its own, larger budget. Workers stay at 8 (see `workers`).
const rtfTestTimeout = Number(process.env.PLAYWRIGHT_RTF_TEST_TIMEOUT ?? 240_000);

const sharedUse = {
  baseURL,
  actionTimeout: Number(process.env.PLAYWRIGHT_ACTION_TIMEOUT ?? 45_000),
  navigationTimeout: Number(process.env.PLAYWRIGHT_NAVIGATION_TIMEOUT ?? 25_000),
  trace: (process.env.PLAYWRIGHT_TRACE as "on" | "off" | "retain-on-failure" | "on-first-retry") ??
    "retain-on-failure",
  locale: "en-US",
};

export default defineConfig({
  timeout: defaultTestTimeout,
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
      testMatch: /save\.spec\.ts$/,
      dependencies: ["chromium"],
      timeout: saveTestTimeout,
      use: { ...devices["Desktop Chrome"] },
    },
    {
      // Two edit→save cycles per test; see rtfTestTimeout.
      name: "chromium-rtf",
      testMatch: /rtf-formatting\.spec\.ts$/,
      dependencies: ["chromium"],
      timeout: rtfTestTimeout,
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
