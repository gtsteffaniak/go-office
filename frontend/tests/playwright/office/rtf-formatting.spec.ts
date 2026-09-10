import { test, expect } from "../test-setup";
import {
  warmDemoFile,
  waitForEditorReady,
  waitForEditorInteractive,
  insertSaveMarker,
  formatWordSelection,
  triggerEditorSave,
  waitForSaveDone,
  assertDemoRtfFormatting,
} from "../editor";
import { forkSample } from "../fork-sample";

test.describe.configure({ mode: "parallel" });

const STATUS_OK_TIMEOUT = 8_000;
const RTF_SOURCE = "sample-files/sample.rtf";

test("rtf bold persists", async ({ page, request }, testInfo) => {
  const marker = "PW_RTF_BOLD_MARKER";
  const file = forkSample(RTF_SOURCE, testInfo);

  await warmDemoFile(request, file);
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  await insertSaveMarker(page, "word", marker);
  await formatWordSelection(page, marker, { bold: true });
  await triggerEditorSave(page, "word");
  await waitForSaveDone(page, { request, filePath: file, marker });

  await assertDemoRtfFormatting(request, file, { marker, bold: true });
});

test("rtf italic persists", async ({ page, request }, testInfo) => {
  const marker = "PW_RTF_ITALIC_MARKER";
  const file = forkSample(RTF_SOURCE, testInfo);

  await warmDemoFile(request, file);
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  await insertSaveMarker(page, "word", marker);
  await formatWordSelection(page, marker, { italic: true });
  await triggerEditorSave(page, "word");
  await waitForSaveDone(page, { request, filePath: file, marker });

  await assertDemoRtfFormatting(request, file, { marker, italic: true });
});

test("rtf highlight persists", async ({ page, request }, testInfo) => {
  const marker = "PW_RTF_HIGHLIGHT_MARKER";
  const file = forkSample(RTF_SOURCE, testInfo);

  await warmDemoFile(request, file);
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  await insertSaveMarker(page, "word", marker);
  await formatWordSelection(page, marker, { highlight: true });
  await triggerEditorSave(page, "word");
  await waitForSaveDone(page, { request, filePath: file, marker });

  await assertDemoRtfFormatting(request, file, { marker, highlight: true });
});
