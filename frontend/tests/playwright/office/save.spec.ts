import { test, expect } from "../test-setup";
import {
  warmDemoFile,
  editCellForSave,
  editWordForSave,
  waitForPersistedMarker,
  triggerManualSave,
  assertDemoFileContains,
} from "../editor";
import { forkSample } from "../fork-sample";

test.describe.configure({ mode: "parallel" });

const STATUS_OK_TIMEOUT = 8_000;

const CSV_SOURCE = "sample-files/sample.csv";
const CSV_CELL = "B2";
const CSV_ORIGINAL = "DD37Cf93aecA6Dc";
const CSV_REPLACEMENT = "REPLACED_VALUE_ID";

const DOCX_SOURCE = "sample-files/sample.docx";
const DOCX_ORIGINAL = "Demonstration of DOCX";
const DOCX_MARKER = "PW_SAVE_DOCX_MARKER";

const TXT_SOURCE = "sample-files/sample.txt";
const TXT_ORIGINAL = "Sample-Files.com";
const TXT_MARKER = "PW_SAVE_TXT_MARKER";

const RTF_SOURCE = "sample-files/sample.rtf";
const RTF_ORIGINAL = "SYSTEM BRIEF & DAILY LOG";

const ODS_SOURCE = "sample-files/sample.ods";
const ODS_CELL = CSV_CELL;
const ODS_ORIGINAL = CSV_ORIGINAL;
const ODS_REPLACEMENT = "REPLACED_ODS_ID";

const PPT_SOURCE = "sample-files/sample.ppt";
const PPT_FIND = "My Presentation";
const PPT_MARKER = "PW_SAVE_PPT_MARKER";

async function openForkedDemo(
  page: import("@playwright/test").Page,
  request: import("@playwright/test").APIRequestContext,
  file: string,
): Promise<void> {
  await warmDemoFile(request, file);
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
}

test("docx save round-trip via Save button", async ({ page, request }, testInfo) => {
  const file = forkSample(DOCX_SOURCE, testInfo);
  await assertDemoFileContains(request, file, DOCX_ORIGINAL);

  await openForkedDemo(page, request, file);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });

  await editWordForSave(page, "word", DOCX_MARKER);
  await triggerManualSave(page, "word");
  await waitForPersistedMarker(request, file, DOCX_MARKER, { page });

  await assertDemoFileContains(request, file, DOCX_MARKER);
});

test("csv save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(CSV_SOURCE, testInfo);
  await assertDemoFileContains(request, file, CSV_ORIGINAL);

  await openForkedDemo(page, request, file);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });

  await editCellForSave(page, "cell", CSV_CELL, CSV_REPLACEMENT);
  await waitForPersistedMarker(request, file, CSV_REPLACEMENT, { page });

  await assertDemoFileContains(request, file, CSV_REPLACEMENT);
  await assertDemoFileContains(request, file, CSV_ORIGINAL, { present: false });
});

test("txt save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(TXT_SOURCE, testInfo);
  await assertDemoFileContains(request, file, TXT_ORIGINAL);

  await openForkedDemo(page, request, file);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });

  await editWordForSave(page, "word", TXT_MARKER);
  await waitForPersistedMarker(request, file, TXT_MARKER, { page });

  await assertDemoFileContains(request, file, TXT_MARKER);
});

test("rtf save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(RTF_SOURCE, testInfo);
  await assertDemoFileContains(request, file, RTF_ORIGINAL);
  await assertDemoFileContains(request, file, "&amp;", { present: false });
  await assertDemoFileContains(request, file, "> Reminder");

  await openForkedDemo(page, request, file);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });

  await editWordForSave(page, "word", "PW_RTF_SAVE_ROUNDTRIP");
  await waitForPersistedMarker(request, file, "PW_RTF_SAVE_ROUNDTRIP", { page });

  await assertDemoFileContains(request, file, "PW_RTF_SAVE_ROUNDTRIP");
  await assertDemoFileContains(request, file, RTF_ORIGINAL);
  await assertDemoFileContains(request, file, "&amp;", { present: false });
  await assertDemoFileContains(request, file, "&gt;", { present: false });
  await assertDemoFileContains(request, file, "> Reminder");
});

test("ods save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(ODS_SOURCE, testInfo);
  await assertDemoFileContains(request, file, ODS_ORIGINAL);

  await openForkedDemo(page, request, file);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });

  await editCellForSave(page, "cell", ODS_CELL, ODS_REPLACEMENT);
  await waitForPersistedMarker(request, file, ODS_REPLACEMENT, { page });

  await assertDemoFileContains(request, file, ODS_REPLACEMENT);
  await assertDemoFileContains(request, file, ODS_ORIGINAL, { present: false });
});

test("ppt save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(PPT_SOURCE, testInfo);
  await assertDemoFileContains(request, file, PPT_FIND);

  await openForkedDemo(page, request, file);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });

  await editWordForSave(page, "slide", PPT_MARKER);
  await waitForPersistedMarker(request, file, PPT_MARKER, { page });

  await assertDemoFileContains(request, file, PPT_MARKER);
});
