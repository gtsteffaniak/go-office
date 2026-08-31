import { test, expect } from "../test-setup";
import { waitForEditorReady, assertSampleContent } from "../editor";
import {
  REPO_ROOT,
  sampleExists,
  type SampleFile,
} from "../samples";
import {
  manifestEntry,
  manifestForTier,
  type SampleManifestEntry,
} from "../fixtures/sample-manifest";

const maxTier = Number(process.env.PLAYWRIGHT_SAMPLE_TIER ?? "3") as 1 | 2 | 3;

const bundledTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const STATUS_OK_TIMEOUT = 8_000;

test.describe.configure({ mode: "parallel" });
test.use({ trace: bundledTest ? "retain-on-failure" : "on-first-retry" });

const coauthoringFailures: string[] = [];

function trackCoauthoring(page: import("@playwright/test").Page) {
  page.on("response", (res) => {
    const url = res.url();
    if (url.includes("/doc/") && url.includes("/c/") && res.request().method() === "POST") {
      if (res.status() >= 300) {
        coauthoringFailures.push(`${res.status()} ${url}`);
      }
    }
  });
}

async function openSample(page: import("@playwright/test").Page, sample: SampleFile) {
  coauthoringFailures.length = 0;
  trackCoauthoring(page);

  await page.goto(`/demo/view?file=${encodeURIComponent(sample.path)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, sample.editor);

  expect(coauthoringFailures, `coauthoring POST failures: ${coauthoringFailures.join(", ")}`).toHaveLength(0);
}

function manifestCases(): SampleManifestEntry[] {
  const tiered = manifestForTier(maxTier);
  const csv = manifestEntry("sample-files/sample.csv");
  const out = [...tiered];
  if (csv && !out.some((e) => e.path === csv.path)) {
    out.push(csv);
  }
  return out.filter((e) => e.text || e.cell?.value || e.editor === "pdf");
}

for (const entry of manifestCases()) {
  test(`content ${entry.path}`, async ({ page }) => {
    if (!sampleExists(entry.path)) {
      throw new Error(
        `missing sample ${entry.path} — commit it under ${REPO_ROOT}/sample-files/`,
      );
    }
    const sample: SampleFile = {
      path: entry.path,
      tier: entry.tier,
      editor: entry.editor,
    };
    await openSample(page, sample);
    await assertSampleContent(page, entry);
  });
}
