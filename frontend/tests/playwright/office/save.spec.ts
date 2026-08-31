import { test, expect } from "../test-setup";
import { waitForEditorReady, waitForEditorInteractive, setCellContent, waitForSaveDone, typeInDocument } from "../editor";

const bundledTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const STATUS_OK_TIMEOUT = 8_000;
const MARKER = `PLAYWRIGHT_EDIT_${Date.now()}`;

test.use({ trace: bundledTest ? "retain-on-failure" : "on-first-retry" });

test("csv save round-trip via demo file API", async ({ page, request }) => {
  const file = "sample-files/sample.csv";
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "cell");
  await waitForEditorInteractive(page, "cell");

  await setCellContent(page, "cell", "B2", MARKER);
  await waitForSaveDone(page, { request, filePath: file, marker: MARKER });

  const res = await request.get(`/api/office/demo/file/${encodeURIComponent(file)}`);
  expect(res.ok()).toBeTruthy();
  const body = await res.text();
  expect(body).toContain(MARKER);
});

test("docx save round-trip via demo file API", async ({ page, request }) => {
  const file = "sample-files/sample.docx";
  const marker = `PLAYWRIGHT_DOCX_${Date.now()}`;
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");
  await typeInDocument(page, "word", marker);
  await waitForSaveDone(page, { request, filePath: file, marker });

  const res = await request.get(`/api/office/demo/file/${encodeURIComponent(file)}`);
  expect(res.ok()).toBeTruthy();
  const buf = await res.body();
  const xml = buf.toString("utf8");
  // DOCX is zip; raw buffer may still contain marker in word/document.xml.
  expect(xml).toContain(marker);
});
