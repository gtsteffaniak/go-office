import { test, expect } from "@playwright/test";

test.describe("demo infrastructure", () => {
  test("health endpoint", async ({ request }) => {
    const res = await request.get("/office/health");
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.status).toBe("ok");
  });

  test("api.js is served", async ({ request }) => {
    const res = await request.get("/office/web-apps/apps/api/documents/api.js");
    expect(res.ok()).toBeTruthy();
    const text = await res.text();
    expect(text.length).toBeGreaterThan(1000);
  });

  test("demo landing page loads", async ({ page }) => {
    await page.goto("/office/demo/");
    await expect(page.getByRole("heading", { name: /go-office demo/i })).toBeVisible();
    await expect(page.locator("code")).first().toBeVisible();
  });
});
