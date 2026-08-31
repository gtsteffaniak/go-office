import { test, expect } from "../test-setup";
import {
  waitForEditorShell,
  waitForDocumentReady,
  setCellContent,
  waitForSaveDone,
} from "../editor";

const bundledTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const STATUS_OK_TIMEOUT = bundledTest ? 15_000 : 8_000;
const MARKER = `PLAYWRIGHT_EDIT_${Date.now()}`;

test.describe.configure({ retries: bundledTest ? 2 : 2 });
test.use({ trace: bundledTest ? "retain-on-failure" : "on-first-retry" });

test("csv save round-trip via demo file API", async ({ page, request }) => {
  const file = "sample-files/sample.csv";
  await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
  await expect(page.locator("#status")).not.toContainText(/^Error:/, {
    timeout: STATUS_OK_TIMEOUT,
  });
  await waitForEditorShell(page, "cell");
  await waitForDocumentReady(page);

  await setCellContent(page, "cell", "B2", MARKER);
  await waitForSaveDone(page, bundledTest ? 45_000 : 30_000);

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
  await waitForEditorShell(page, "word");
  await waitForDocumentReady(page);

  const frame = page.frameLocator('iframe[src*="/documenteditor/"]').first();
  await frame.locator("#id_main, #editor_sdk").first().click();
  await page.keyboard.type(marker);
  await waitForSaveDone(page, bundledTest ? 45_000 : 30_000);

  const res = await request.get(`/api/office/demo/file/${encodeURIComponent(file)}`);
  expect(res.ok()).toBeTruthy();
  const buf = await res.body();
  const xml = buf.toString("utf8");
  // DOCX is zip; raw buffer may still contain marker in word/document.xml.
  expect(xml).toContain(marker);
});
