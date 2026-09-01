import { expect, type Page } from "@playwright/test";

const CANONICAL_SAMPLES = [
  "sample-files/sample.csv",
  "sample-files/sample.docx",
  "sample-files/sample.txt",
];

export async function gotoDemoLanding(page: Page): Promise<void> {
  await page.goto("/demo/", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("heading", { name: /go-office demo/i })).toBeVisible();
}

/** Canonical sample rows expose populated thumbnail img tags (DOM only, no decode/API wait). */
export async function expectSampleThumbnailsListed(page: Page): Promise<void> {
  for (const samplePath of CANONICAL_SAMPLES) {
    const row = page.locator("li", {
      has: page.locator(`a[href="/demo/view?file=${encodeURIComponent(samplePath)}"]`),
    });
    await expect(row, samplePath).toBeVisible();

    const img = row.locator("img.thumb[data-sample-thumb]");
    await expect(img).toBeVisible();

    const src = await img.getAttribute("src");
    expect(src, `${samplePath} src`).toBeTruthy();
    expect(src).toContain("/api/office/demo/thumbnail?file=");
    expect(decodeURIComponent(src!.split("file=")[1] ?? "")).toBe(samplePath);
  }
}
