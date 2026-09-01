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
import { forkSample } from "../fork-sample";

const bundledTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const STATUS_OK_TIMEOUT = 8_000;

const CSV_SOURCE = "sample-files/sample.csv";
const CSV_CELL = "B2";
const CSV_ORIGINAL = "DD37Cf93aecA6Dc";
const CSV_REPLACEMENT = "REPLACED_VALUE_ID";

const DOCX_SOURCE = "sample-files/sample.docx";
const DOCX_ORIGINAL = "Demonstration of DOCX";
const DOCX_FIND = "DOCX";
const DOCX_REPLACEMENT = "REPLACED";
const DOCX_EXPECTED = "Demonstration of REPLACED";

const TXT_SOURCE = "sample-files/sample.txt";
const TXT_ORIGINAL = "Sample-Files.com";
const TXT_FIND = "Sample-Files";
const TXT_REPLACEMENT = "REPLACED-SOURCE";
const TXT_EXPECTED = "REPLACED-SOURCE.com";

test.use({ trace: bundledTest ? "retain-on-failure" : "on-first-retry" });

test("docx save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(DOCX_SOURCE, testInfo);
  await assertDemoFileContains(request, file, DOCX_ORIGINAL);

  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  await replaceDocumentText(page, "word", DOCX_FIND, DOCX_REPLACEMENT);
  await triggerEditorSave(page, "word");
  await waitForSaveDone(page, {
    request,
    filePath: file,
    marker: DOCX_EXPECTED,
  });

  await assertDemoFileContains(request, file, DOCX_EXPECTED);
});

test("csv save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(CSV_SOURCE, testInfo);
  await assertDemoFileContains(request, file, CSV_ORIGINAL);

  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "cell");
  await waitForEditorInteractive(page, "cell");

  await setCellContent(page, "cell", CSV_CELL, CSV_REPLACEMENT);
  await triggerEditorSave(page, "cell");
  await waitForSaveDone(page, {
    request,
    filePath: file,
    marker: CSV_REPLACEMENT,
  });

  await assertDemoFileContains(request, file, CSV_REPLACEMENT);
  await assertDemoFileContains(request, file, CSV_ORIGINAL, { present: false });
});

test("txt save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(TXT_SOURCE, testInfo);
  await assertDemoFileContains(request, file, TXT_ORIGINAL);

  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  await replaceDocumentText(page, "word", TXT_FIND, TXT_REPLACEMENT);
  await triggerEditorSave(page, "word");
  await waitForSaveDone(page, {
    request,
    filePath: file,
    marker: TXT_EXPECTED,
  });

  await assertDemoFileContains(request, file, TXT_EXPECTED);
  await assertDemoFileContains(request, file, TXT_ORIGINAL, { present: false });
});
