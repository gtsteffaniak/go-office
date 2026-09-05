import { test, expect } from "../test-setup";
import {
  waitForEditorReady,
  waitForEditorInteractive,
  applyMinimalSaveEdit,
  triggerEditorSave,
  waitForSaveDone,
  assertEditorStable,
} from "../editor";
import { forkSample } from "../fork-sample";
import { samplesForTier, sampleExists } from "../samples";

test.describe.configure({ mode: "parallel" });

const STATUS_OK_TIMEOUT = 8_000;
const STABLE_MS = Number(process.env.POST_SAVE_STABLE_MS ?? 10_000);
const SAVE_TEST_TIMEOUT = Number(process.env.PLAYWRIGHT_SAVE_TEST_TIMEOUT ?? 150_000);

const STABLE_SAMPLES = samplesForTier(3).filter((s) => s.editor !== "pdf" && sampleExists(s.path));

test.use({
  trace: "on-first-retry",
  timeout: SAVE_TEST_TIMEOUT,
});

for (const sample of STABLE_SAMPLES) {
  test(`post-save stable: ${sample.path}`, async ({ page }, testInfo) => {
    const marker = `PW_STABLE_${testInfo.testId.slice(-6)}`;
    const file = forkSample(sample.path, testInfo);

    await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
    await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });
    await waitForEditorReady(page, sample.editor);
    await waitForEditorInteractive(page, sample.editor);

    await applyMinimalSaveEdit(page, sample.editor, marker);
    await triggerEditorSave(page, sample.editor);
    await waitForSaveDone(page, { request: page.request, filePath: file, marker });

    await assertEditorStable(page, STABLE_MS);
  });
}
