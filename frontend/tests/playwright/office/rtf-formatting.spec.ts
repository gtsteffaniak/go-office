import { test, expect } from "../test-setup";
import {
  warmDemoFile,
  editWordForSave,
  formatWordSelection,
  waitForPersistedMarker,
  waitForDemoRtfFormatting,
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

  await editWordForSave(page, "word", marker);
  await waitForPersistedMarker(request, file, marker, { page });
  await formatWordSelection(page, marker, { bold: true });
  await waitForDemoRtfFormatting(request, file, { marker, bold: true });

  await assertDemoRtfFormatting(request, file, { marker, bold: true });
});

test("rtf italic persists", async ({ page, request }, testInfo) => {
  const marker = "PW_RTF_ITALIC_MARKER";
  const file = forkSample(RTF_SOURCE, testInfo);

  await warmDemoFile(request, file);
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });

  await editWordForSave(page, "word", marker);
  await waitForPersistedMarker(request, file, marker, { page });
  await formatWordSelection(page, marker, { italic: true });
  await waitForDemoRtfFormatting(request, file, { marker, italic: true });

  await assertDemoRtfFormatting(request, file, { marker, italic: true });
});

test("rtf highlight persists", async ({ page, request }, testInfo) => {
  const marker = "PW_RTF_HIGHLIGHT_MARKER";
  const file = forkSample(RTF_SOURCE, testInfo);

  await warmDemoFile(request, file);
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, { timeout: STATUS_OK_TIMEOUT });

  await editWordForSave(page, "word", marker);
  await waitForPersistedMarker(request, file, marker, { page });
  await formatWordSelection(page, marker, { highlight: true });
  await waitForDemoRtfFormatting(request, file, { marker, highlight: true });

  await assertDemoRtfFormatting(request, file, { marker, highlight: true });
});
