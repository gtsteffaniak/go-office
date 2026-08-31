import {
  expect,
  type APIRequestContext,
  type FrameLocator,
  type Page,
} from "@playwright/test";
import type { SampleFile } from "./samples";
import type { SampleManifestEntry } from "./fixtures/sample-manifest";

/** Euro-Office web-apps path segment per editor kind. */
const EDITOR_APP: Record<SampleFile["editor"], string> = {
  word: "documenteditor",
  cell: "spreadsheeteditor",
  slide: "presentationeditor",
  pdf: "pdfeditor",
};

/** Shell elements inside the editor app frame (not on the demo viewer page). */
const EDITOR_SHELL = "#editor-container, #id_main, #editor_sdk, #id_view";

const bundledTest = process.env.OFFICE_PLAYWRIGHT_TEST === "true";
const EDITOR_LOAD_TIMEOUT = Number(
  process.env.PLAYWRIGHT_EDITOR_TIMEOUT ?? (bundledTest ? 25_000 : 30_000),
);
const DOCUMENT_READY_TIMEOUT = Number(
  process.env.PLAYWRIGHT_DOCUMENT_READY_TIMEOUT ?? (bundledTest ? 60_000 : 45_000),
);
const CONTENT_FIND_TIMEOUT = Number(
  process.env.PLAYWRIGHT_CONTENT_FIND_TIMEOUT ?? (bundledTest ? 15_000 : 15_000),
);
const SAVE_DONE_TIMEOUT = Number(
  process.env.PLAYWRIGHT_SAVE_DONE_TIMEOUT ?? (bundledTest ? 25_000 : 30_000),
);
const INTERACTIVE_SETTLE_MS = 400;

const LOAD_MASK_SELECTORS = [
  ".asc-loadmask",
  ".asc-loader-mask",
  ".asc-plugin-loader",
  "#loading-mask",
  ".loadmask",
  ".asc-loadmask-body",
];

type AscEditor = {
  asc_selectRange?: (ref: string) => void;
  asc_getCellText?: () => string;
  asc_getFormula?: () => string;
  asc_setCellValue?: (value: string) => void;
  asc_insertText?: (text: string) => void;
  asc_closeCellEditor?: (save: boolean) => void;
  asc_findText?: (text: string, matchCase: boolean, wholeCell: boolean) => boolean;
  asc_isDocumentCanSave?: () => boolean;
};
export function getEditorFrame(page: Page, editor: SampleFile["editor"]): FrameLocator {
  const app = EDITOR_APP[editor];
  return page.frameLocator(`iframe[src*="/${app}/"]`).first();
}

async function isLoadMaskBlocking(frame: FrameLocator): Promise<boolean> {
  return frame.locator("body").evaluate((_, selectors: string[]) => {
    const isMaskNode = (node: Element | null): boolean => {
      if (!node) {
        return false;
      }
      for (const sel of selectors) {
        if (node.matches(sel) || node.closest(sel)) {
          return true;
        }
      }
      return false;
    };

    const probePoints: Array<{ x: number; y: number }> = [];
    for (const id of ["#editor_sdk", "#id_main", "#editor-container", "#ce-cell-name"]) {
      const el = document.querySelector(id);
      if (!el) {
        continue;
      }
      const rect = el.getBoundingClientRect();
      if (rect.width < 2 || rect.height < 2) {
        continue;
      }
      probePoints.push({
        x: rect.left + rect.width / 2,
        y: rect.top + rect.height / 2,
      });
    }

    for (const point of probePoints) {
      const hit = document.elementFromPoint(point.x, point.y);
      if (isMaskNode(hit)) {
        return true;
      }
    }

    for (const sel of selectors) {
      for (const el of document.querySelectorAll(sel)) {
        const style = window.getComputedStyle(el);
        if (style.display === "none" || style.visibility === "hidden") {
          continue;
        }
        if (style.pointerEvents === "none") {
          continue;
        }
        const opacity = Number.parseFloat(style.opacity);
        if (!Number.isNaN(opacity) && opacity === 0) {
          continue;
        }
        const rect = el.getBoundingClientRect();
        if (rect.width > 1 && rect.height > 1) {
          return true;
        }
      }
    }
    return false;
  }, LOAD_MASK_SELECTORS);
}

async function isEditorShellReady(
  frame: FrameLocator,
  editor: SampleFile["editor"],
): Promise<boolean> {
  if (editor === "pdf") {
    const view = frame.locator("#id_view, #id_main").first();
    if (!(await view.isVisible())) {
      return false;
    }
    const box = await view.boundingBox();
    return box !== null && box.height > 50;
  }

  const shell = frame.locator(EDITOR_SHELL).first();
  if (!(await shell.isVisible())) {
    return false;
  }
  const box = await shell.boundingBox();
  return box !== null && box.width > 50 && box.height > 50;
}

async function isEditorInteractive(
  page: Page,
  frame: FrameLocator,
  editor: SampleFile["editor"],
): Promise<boolean> {
  if (await isLoadMaskBlocking(frame)) {
    return false;
  }

  if (editor === "cell") {
    const cellName = frame.locator("#ce-cell-name").first();
    if ((await cellName.count()) === 0) {
      return false;
    }
    if (!(await cellName.isEnabled())) {
      return false;
    }
    return frame.locator("body").evaluate(() => {
      const api = (window as { Asc?: { editor?: AscEditor } }).Asc?.editor;
      return typeof api?.asc_selectRange === "function";
    });
  }

  const parentReady =
    (await page.locator("body").getAttribute("data-document-ready")) === "true";
  if (parentReady) {
    return true;
  }

  return frame.locator("body").evaluate(() => {
    const api = (window as { Asc?: { editor?: AscEditor } }).Asc?.editor;
    return api?.asc_isDocumentCanSave?.() ?? false;
  });
}

/**
 * DocsAPI mounts the Euro-Office editor in an app iframe (document/cell/slide/pdf).
 * The shell divs (#id_main, #editor-container, …) live inside that frame.
 */
export async function waitForEditorShell(
  page: Page,
  editor: SampleFile["editor"],
): Promise<void> {
  const appFrame = getEditorFrame(page, editor);
  await expect(appFrame.locator(EDITOR_SHELL).first()).toBeVisible({
    timeout: EDITOR_LOAD_TIMEOUT,
  });
}

/** Editor iframe mounted and shell has usable dimensions (open-format tests). */
export async function waitForEditorReady(
  page: Page,
  editor: SampleFile["editor"],
): Promise<void> {
  await waitForEditorShell(page, editor);
  const frame = getEditorFrame(page, editor);
  await expect
    .poll(async () => isEditorShellReady(frame, editor), { timeout: EDITOR_LOAD_TIMEOUT })
    .toBe(true);
}

/** Document loaded, load masks gone, and editor APIs are usable (content/save tests). */
export async function waitForEditorInteractive(
  page: Page,
  editor: SampleFile["editor"],
  timeoutMs = DOCUMENT_READY_TIMEOUT,
): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await expect
    .poll(async () => isEditorInteractive(page, frame, editor), { timeout: timeoutMs })
    .toBe(true);
  await page.waitForTimeout(INTERACTIVE_SETTLE_MS);
  if (await isLoadMaskBlocking(frame)) {
    await expect
      .poll(async () => isEditorInteractive(page, frame, editor), { timeout: 10_000 })
      .toBe(true);
  }
}

/** @deprecated Use waitForEditorReady — body data-document-ready is unreliable under load. */
export async function waitForDocumentReady(
  page: Page,
  editor: SampleFile["editor"],
): Promise<void> {
  await waitForEditorReady(page, editor);
  await waitForEditorInteractive(page, editor);
}

async function readCellValue(frame: FrameLocator, ref: string): Promise<string> {
  return frame.locator("body").evaluate((_, cellRef: string) => {
    const api = (window as { Asc?: { editor?: AscEditor } }).Asc?.editor;
    if (!api?.asc_selectRange) {
      return "";
    }
    api.asc_selectRange(cellRef);
    return api.asc_getCellText?.() ?? api.asc_getFormula?.() ?? "";
  }, ref);
}

async function setCellValue(frame: FrameLocator, ref: string, value: string): Promise<boolean> {
  return frame.locator("body").evaluate(
    ({ cellRef, cellValue }) => {
      const api = (window as { Asc?: { editor?: AscEditor } }).Asc?.editor;
      if (!api?.asc_selectRange) {
        return false;
      }
      api.asc_selectRange(cellRef);
      if (typeof api.asc_setCellValue === "function") {
        api.asc_setCellValue(cellValue);
        return true;
      }
      if (typeof api.asc_insertText === "function") {
        api.asc_insertText(cellValue);
        api.asc_closeCellEditor?.(true);
        return true;
      }
      return false;
    },
    { cellRef: ref, cellValue: value },
  );
}

export async function assertCellContent(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  expected: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);
  const frame = getEditorFrame(page, editor);
  const value = await readCellValue(frame, ref);
  expect(value, `cell ${ref} value`).toContain(expected);
}

export async function assertDocumentContains(
  page: Page,
  editor: SampleFile["editor"],
  text: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);
  const frame = getEditorFrame(page, editor);

  const foundViaSdk = await frame.locator("body").evaluate((_, needle: string) => {
    const api = (window as { Asc?: { editor?: AscEditor } }).Asc?.editor;
    return api?.asc_findText?.(needle, false, false) ?? false;
  }, text);
  if (foundViaSdk) {
    return;
  }

  await page.keyboard.press("Control+KeyF");
  const findInput = frame.locator('input[type="search"], input[placeholder*="Find" i]').first();
  if ((await findInput.count()) > 0) {
    await expect(findInput).toBeVisible({ timeout: CONTENT_FIND_TIMEOUT });
    await findInput.fill(text);
    await expect(frame.getByText(text).first()).toBeVisible({ timeout: CONTENT_FIND_TIMEOUT });
    await page.keyboard.press("Escape");
    return;
  }

  const hasText = await frame.locator("body").evaluate((body, needle: string) => {
    return body.innerText.includes(needle);
  }, text);
  expect(hasText, `document should contain ${text}`).toBe(true);
}

export async function setCellContent(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  value: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);
  const frame = getEditorFrame(page, editor);
  const ok = await setCellValue(frame, ref, value);
  if (ok) {
    return;
  }

  const cellName = frame.locator("#ce-cell-name").first();
  await expect(cellName).toBeEnabled({ timeout: EDITOR_LOAD_TIMEOUT });
  await expect.poll(async () => !(await isLoadMaskBlocking(frame)), { timeout: DOCUMENT_READY_TIMEOUT }).toBe(true);
  await cellName.focus();
  await cellName.fill(ref);
  await cellName.press("Enter");
  await expect.poll(async () => !(await isLoadMaskBlocking(frame)), { timeout: DOCUMENT_READY_TIMEOUT }).toBe(true);
  await cellName.focus();
  await cellName.fill(value);
  await cellName.press("Enter");
}

/** Insert text into the active document without clicking through load masks. */
export async function typeInDocument(
  page: Page,
  editor: SampleFile["editor"],
  text: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);
  const frame = getEditorFrame(page, editor);
  const inserted = await frame.locator("body").evaluate((_, chunk: string) => {
    const api = (window as { Asc?: { editor?: AscEditor } }).Asc?.editor;
    if (!api?.asc_insertText) {
      return false;
    }
    api.asc_insertText(chunk);
    return true;
  }, text);
  if (inserted) {
    return;
  }

  await expect.poll(async () => !(await isLoadMaskBlocking(frame)), { timeout: DOCUMENT_READY_TIMEOUT }).toBe(true);
  await frame.locator("#id_main, #editor_sdk, #editor-container").first().click();
  await page.keyboard.type(text);
}

export type SaveDoneOptions = {
  timeoutMs?: number;
  request?: APIRequestContext;
  filePath?: string;
  marker?: string;
};

export async function waitForSaveDone(
  page: Page,
  opts?: SaveDoneOptions | number,
): Promise<void> {
  const options: SaveDoneOptions =
    typeof opts === "number" ? { timeoutMs: opts } : (opts ?? {});
  const timeoutMs = options.timeoutMs ?? SAVE_DONE_TIMEOUT;

  await expect
    .poll(
      async () => {
        if (options.marker && options.filePath && options.request) {
          const res = await options.request.get(
            `/api/office/demo/file/${encodeURIComponent(options.filePath)}`,
          );
          if (res.ok()) {
            const body = await res.body();
            const text = new TextDecoder().decode(body);
            if (text.includes(options.marker)) {
              return true;
            }
          }
        }
        const attr = await page.locator("body").getAttribute("data-save-done");
        return attr !== null && attr !== "";
      },
      { timeout: timeoutMs },
    )
    .toBe(true);
}

export async function assertSampleContent(
  page: Page,
  sample: SampleManifestEntry,
): Promise<void> {
  if (sample.cell?.value) {
    await assertCellContent(page, sample.editor, sample.cell.ref, sample.cell.value);
    return;
  }
  if (sample.text) {
    await assertDocumentContains(page, sample.editor, sample.text);
    return;
  }
  if (sample.editor === "pdf") {
    const frame = getEditorFrame(page, sample.editor);
    await expect(frame.locator("#id_main, #editor_sdk").first()).toBeVisible();
  }
}
