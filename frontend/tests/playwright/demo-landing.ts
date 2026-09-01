import { expect, type APIRequestContext, type Page } from "@playwright/test";

const THUMB_SELECTOR = "img[data-sample-thumb]";

export async function gotoDemoLanding(page: Page): Promise<void> {
  await page.goto("/demo/", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("heading", { name: /go-office demo/i })).toBeVisible();
}

/** Every sample row should expose a thumbnail img with a populated demo thumbnail URL. */
export async function expectSampleThumbnailsListed(page: Page): Promise<void> {
  const thumbs = page.locator(THUMB_SELECTOR);
  await expect(thumbs.first()).toBeVisible();

  const count = await thumbs.count();
  expect(count).toBeGreaterThan(0);
  expect(count).toBe(await page.locator("ul li").count());

  for (let i = 0; i < count; i++) {
    const src = await thumbs.nth(i).getAttribute("src");
    expect(src, `thumbnail ${i} src`).toBeTruthy();
    expect(src).toContain("/api/office/demo/thumbnail?file=");
    expect(src).not.toMatch(/^data:/);
  }
}

/** Spot-check a few thumbnail endpoints return image bytes (no browser decode wait). */
export async function expectThumbnailEndpointsOK(
  request: APIRequestContext,
  page: Page,
  sampleCount = 3,
): Promise<void> {
  const thumbs = page.locator(THUMB_SELECTOR);
  const total = await thumbs.count();
  const checks = Math.min(sampleCount, total);
  expect(checks).toBeGreaterThan(0);

  for (let i = 0; i < checks; i++) {
    const src = await thumbs.nth(i).getAttribute("src");
    expect(src).toBeTruthy();

    const res = await request.get(src!);
    expect(res.ok(), `thumbnail ${i} ${src}`).toBeTruthy();

    const contentType = res.headers()["content-type"] ?? "";
    expect(contentType, `thumbnail ${i} content-type`).toMatch(/^image\//);

    const body = await res.body();
    expect(body.byteLength, `thumbnail ${i} body`).toBeGreaterThan(0);
  }
}
