import { test, expect } from "../test-setup";
import {
  waitForEditorReady,
  waitForEditorInteractive,
  setCellContent,
  waitForSaveDone,
  replaceDocumentText,
  triggerEditorSave,
  assertDemoFileContains,
  discoverSlideMarker,
  fetchDemoFileBody,
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

const RTF_SOURCE = "sample-files/sample.rtf";
const RTF_ORIGINAL = "Lorem ipsum dolor sit amet";
const RTF_FIND = "ipsum";
const RTF_REPLACEMENT = "REPLACED";
const RTF_EXPECTED = "Lorem REPLACED dolor sit amet";

const ODS_SOURCE = "sample-files/sample.ods";
const ODS_CELL = CSV_CELL;
const ODS_ORIGINAL = CSV_ORIGINAL;
const ODS_REPLACEMENT = "REPLACED_ODS_ID";

const PPT_SOURCE = "sample-files/sample.ppt";

const SAVE_TEST_TIMEOUT = Number(process.env.PLAYWRIGHT_SAVE_TEST_TIMEOUT ?? (bundledTest ? 150_000 : 120_000));

test.describe.configure({ mode: "serial" });
test.use({
  trace: bundledTest ? "retain-on-failure" : "on-first-retry",
  timeout: SAVE_TEST_TIMEOUT,
});

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

test("rtf save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(RTF_SOURCE, testInfo);
  await assertDemoFileContains(request, file, RTF_ORIGINAL);

  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  await replaceDocumentText(page, "word", RTF_FIND, RTF_REPLACEMENT);
  await triggerEditorSave(page, "word");
  await waitForSaveDone(page, {
    request,
    filePath: file,
    marker: RTF_EXPECTED,
  });

  await assertDemoFileContains(request, file, RTF_EXPECTED);
});

test("ods save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(ODS_SOURCE, testInfo);
  await assertDemoFileContains(request, file, ODS_ORIGINAL);

  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "cell");
  await waitForEditorInteractive(page, "cell");

  await setCellContent(page, "cell", ODS_CELL, ODS_REPLACEMENT);
  await triggerEditorSave(page, "cell");
  await waitForSaveDone(page, {
    request,
    filePath: file,
    marker: ODS_REPLACEMENT,
  });

  await assertDemoFileContains(request, file, ODS_REPLACEMENT);
  await assertDemoFileContains(request, file, ODS_ORIGINAL, { present: false });
});

test("ppt save round-trip via demo file API", async ({ page, request }, testInfo) => {
  const file = forkSample(PPT_SOURCE, testInfo);
  const initial = await fetchDemoFileBody(request, file);
  const marker = discoverSlideMarker(initial);
  const replacement = "REPLACED_SLIDE";

  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "slide");
  await waitForEditorInteractive(page, "slide");

  await replaceDocumentText(page, "slide", marker, replacement);
  await triggerEditorSave(page, "slide");
  await waitForSaveDone(page, {
    request,
    filePath: file,
    marker: replacement,
  });

  await assertDemoFileContains(request, file, replacement);
  await assertDemoFileContains(request, file, marker, { present: false });
});
