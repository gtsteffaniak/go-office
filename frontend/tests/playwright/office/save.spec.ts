import { test, expect } from "../test-setup";
import {
  waitForEditorReady,
  waitForEditorInteractive,
  setCellContent,
  waitForSaveDone,
  replaceDocumentText,
  triggerEditorSave,
  assertDemoFileContains,
} from "../editor";

const bundledTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const STATUS_OK_TIMEOUT = 8_000;

const CSV_FILE = "sample-files/sample.csv";
const CSV_CELL = "B2";
const CSV_ORIGINAL = "DD37Cf93aecA6Dc";
const CSV_REPLACEMENT = "REPLACED_VALUE_ID";

const DOCX_FILE = "sample-files/sample.docx";
const DOCX_ORIGINAL = "Demonstration of DOCX";
const DOCX_REPLACEMENT = "REPLACED_DOCX_TITLE";

test.describe.configure({ mode: "serial" });
test.use({ trace: bundledTest ? "retain-on-failure" : "on-first-retry" });

test("docx save round-trip via demo file API", async ({ page, request }) => {
  await assertDemoFileContains(request, DOCX_FILE, DOCX_ORIGINAL);

  await page.goto(`/demo/view?file=${encodeURIComponent(DOCX_FILE)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  await replaceDocumentText(page, "word", DOCX_ORIGINAL, DOCX_REPLACEMENT);
  await triggerEditorSave(page, "word");
  await waitForSaveDone(page, {
    request,
    filePath: DOCX_FILE,
    marker: DOCX_REPLACEMENT,
  });

  await assertDemoFileContains(request, DOCX_FILE, DOCX_REPLACEMENT);
  await assertDemoFileContains(request, DOCX_FILE, DOCX_ORIGINAL, { present: false });
});

test("csv save round-trip via demo file API", async ({ page, request }) => {
  await assertDemoFileContains(request, CSV_FILE, CSV_ORIGINAL);

  await page.goto(`/demo/view?file=${encodeURIComponent(CSV_FILE)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "cell");
  await waitForEditorInteractive(page, "cell");

  await setCellContent(page, "cell", CSV_CELL, CSV_REPLACEMENT);
  await triggerEditorSave(page, "cell");
  await waitForSaveDone(page, {
    request,
    filePath: CSV_FILE,
    marker: CSV_REPLACEMENT,
  });

  await assertDemoFileContains(request, CSV_FILE, CSV_REPLACEMENT);
  await assertDemoFileContains(request, CSV_FILE, CSV_ORIGINAL, { present: false });
});
