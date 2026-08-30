import { test, expect } from "../test-setup";
import {
  REPO_ROOT,
  sampleExists,
  samplesForTier,
  type SampleFile,
} from "../samples";

const maxTier = Number(process.env.PLAYWRIGHT_SAMPLE_TIER ?? "3") as 1 | 2 | 3;

async function assertEditorOpens(page: import("@playwright/test").Page, sample: SampleFile) {
  const font404s: string[] = [];
  page.on("response", (res) => {
    const url = res.url();
    if (url.includes("/office/fonts/") && res.status() === 404) {
      font404s.push(url);
    }
  });

  await page.goto(`/office/demo/view?file=${encodeURIComponent(sample.path)}`);

  await expect(page.locator("#sample-path")).toContainText(sample.path.split("/").pop()!);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: 15_000 });

  // DocsAPI mounts an iframe for the editor chrome.
  await expect(page.locator("#editor iframe")).toBeVisible({ timeout: 90_000 });

  // Allow coauthoring + font fetches to settle.
  await page.waitForTimeout(3_000);

  expect(font404s, `font 404s: ${font404s.join(", ")}`).toHaveLength(0);
  await expect(page.locator("#status")).not.toContainText(/fonts are not loaded/i);
}

for (const sample of samplesForTier(maxTier)) {
  test(`opens ${sample.path} (${sample.editor})`, async ({ page }) => {
    if (!sampleExists(sample.path)) {
      throw new Error(
        `missing sample ${sample.path} — commit it under ${REPO_ROOT}/sample-files/ (see make check-sample-matrix)`,
      );
    }
    await assertEditorOpens(page, sample);
  });
}
