import { test, expect } from "../test-setup";
import {
  warmDemoFile,
  applyMinimalSaveEdit,
  assertEditorStable,
  describeCellEdit,
  expectPersistedMarker,
} from "../editor";
import { forkSample } from "../fork-sample";
import { samplesForTier, sampleExists } from "../samples";

test.describe.configure({ mode: "parallel" });

const STATUS_OK_TIMEOUT = 8_000;
const STABLE_MS = Number(process.env.POST_SAVE_STABLE_MS ?? 10_000);

const STABLE_SAMPLES = samplesForTier(3).filter((s) => s.editor !== "pdf" && sampleExists(s.path));
for (const sample of STABLE_SAMPLES) {
  test(`post-save stable: ${sample.path}`, async ({ page, request }, testInfo) => {
    const marker = `PW_STABLE_${testInfo.testId.slice(-6)}`;
    const file = forkSample(sample.path, testInfo);

    await warmDemoFile(request, file);
    await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
    await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });

    // The edit is applied best-effort; what this suite asserts is that the change persists
    // and the editor stays stable afterwards. Editor-side acknowledgement is unreliable for
    // the cell editor under parallel CI load (see editCellForSave), so the stored file is
    // the contract. The outcome is kept for diagnostics if the marker never lands.
    const edit = await applyMinimalSaveEdit(page, sample.editor, marker);
    const editDetail = edit
      ? `cell edit: ${describeCellEdit(edit)}`
      : "edit: word/slide append (no editor-side acknowledgement required)";

    await expectPersistedMarker(request, file, marker, page, editDetail);

    await assertEditorStable(page, STABLE_MS);
  });
}

test("post-save rapid double save: sample.docx", async ({ page, request }, testInfo) => {
  const file = forkSample("sample-files/sample.docx", testInfo);
  const markerA = `PW_RAPID_A_${testInfo.testId.slice(-4)}`;
  const markerB = `PW_RAPID_B_${testInfo.testId.slice(-4)}`;

  await warmDemoFile(request, file);
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });

  const editA = await applyMinimalSaveEdit(page, "word", markerA);
  await expectPersistedMarker(
    request,
    file,
    markerA,
    page,
    editA ? `cell edit: ${describeCellEdit(editA)}` : "edit: word append (A)",
  );

  const editB = await applyMinimalSaveEdit(page, "word", markerB);
  await expectPersistedMarker(
    request,
    file,
    markerB,
    page,
    editB ? `cell edit: ${describeCellEdit(editB)}` : "edit: word append (B)",
  );

  await assertEditorStable(page, STABLE_MS);
});
