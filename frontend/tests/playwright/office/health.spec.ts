import { test, expect } from "../test-setup";

test.describe.configure({ mode: "parallel" });

test.describe("demo infrastructure", () => {
  test("health endpoint", async ({ request }) => {
    const res = await request.get("/health");
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.status).toBe("ok");
  });

  test("healthcheck endpoint", async ({ request }) => {
    const res = await request.get("/healthcheck");
    expect(res.ok()).toBeTruthy();
    expect(await res.text()).toBe("true");
  });

  test("api.js is served", async ({ request }) => {
    const res = await request.get("/web-apps/apps/api/documents/api.js");
    expect(res.ok()).toBeTruthy();
    const text = await res.text();
    expect(text.length).toBeGreaterThan(1000);
  });

  test("demo landing page loads", async ({ page }) => {
    await page.goto("/demo/", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: /go-office demo/i })).toBeVisible();
    await expect(page.locator('a[href="/demo/view?file=sample-files%2Fsample.csv"]')).toBeVisible();
    await expect(
      page.locator('a[href="/demo/view?file=sample-files%2Fsample.csv"] img.thumb[data-sample-thumb]'),
    ).toBeVisible();
  });

  test("site home loads", async ({ page }) => {
    await page.goto("/", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: /^go-office$/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /github.com\/quantumx-apps\/go-office/i })).toBeVisible();
  });
});
