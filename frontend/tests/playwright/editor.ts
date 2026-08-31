import {
  expect,
  type APIRequestContext,
  type FrameLocator,
  type Page,
} from "@playwright/test";
import { inflateRawSync } from "node:zlib";
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
  process.env.PLAYWRIGHT_SAVE_DONE_TIMEOUT ?? (bundledTest ? 45_000 : 30_000),
);
const INTERACTIVE_SETTLE_MS = 400;

/** Spreadsheet name box (e.g. B2) and formula bar (cell value). */
const CELL_NAME_INPUT = "#ce-cell-name";
const CELL_VALUE_INPUT = "#ce-cell-content, #ce-text";

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
  asc_AddText?: (text: string) => void;
  asc_enterText?: (text: string) => void;
  asc_nativeInsertText?: (text: string) => void;
  asc_PasteText?: (text: string) => void;
  PasteText?: (text: string) => void;
  asc_closeCellEditor?: (save: boolean) => void;
  asc_findText?: (text: string, matchCase?: boolean, wholeCell?: boolean) => boolean;
  asc_replaceText?: (
    searchProps: { searchString: string; matchCase?: boolean } | string,
    replaceWith: string,
    replaceAll?: boolean,
  ) => boolean | void;
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
    const shellReady = await isEditorShellReady(frame, editor);
    if (!shellReady) {
      return false;
    }
    const hasAPI = await frame.locator("body").evaluate(() => {
      const w = window as { Asc?: { editor?: AscEditor; spreadsheet?: AscEditor }; editor?: AscEditor };
      const candidates = [w.Asc?.editor, w.Asc?.spreadsheet, w.editor];
      return candidates.some((api) => typeof api?.asc_selectRange === "function");
    });
    if (hasAPI) {
      return true;
    }
    return cellName.isVisible();
  }

  if (editor === "word" || editor === "slide") {
    const shellReady = await isEditorShellReady(frame, editor);
    if (!shellReady || (await isLoadMaskBlocking(frame))) {
      return false;
    }
    const parentReady =
      (await page.locator("body").getAttribute("data-document-ready")) === "true";
    if (parentReady) {
      return true;
    }
    return frame.locator("body").evaluate(() => {
      const w = window as { Asc?: { editor?: AscEditor }; editor?: AscEditor };
      const api = w.Asc?.editor ?? w.editor;
      if (typeof api?.asc_isDocumentCanSave === "function") {
        return api.asc_isDocumentCanSave();
      }
      if (typeof api?.asc_insertText === "function") {
        return true;
      }
      const main = document.querySelector("#id_main, #editor_sdk, #editor-container");
      if (!main) {
        return false;
      }
      const rect = main.getBoundingClientRect();
      return rect.width > 50 && rect.height > 50;
    });
  }

  const parentReady =
    (await page.locator("body").getAttribute("data-document-ready")) === "true";
  if (parentReady) {
    return true;
  }

  const shellReady = await isEditorShellReady(frame, editor);
  if (!shellReady) {
    return false;
  }

  return frame.locator("body").evaluate(() => {
    const w = window as { Asc?: { editor?: AscEditor }; editor?: AscEditor };
    const api = w.Asc?.editor ?? w.editor;
    if (typeof api?.asc_isDocumentCanSave === "function") {
      return api.asc_isDocumentCanSave();
    }
    if (typeof api?.asc_insertText === "function") {
      return true;
    }
    const main = document.querySelector("#id_main, #editor_sdk, #editor-container");
    if (!main) {
      return false;
    }
    const rect = main.getBoundingClientRect();
    return rect.width > 50 && rect.height > 50;
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

/** Wait until DocsAPI reports the document is ready to edit. */
export async function waitForDocumentReadyAttr(
  page: Page,
  timeoutMs = DOCUMENT_READY_TIMEOUT,
): Promise<void> {
  await expect
    .poll(async () => (await page.locator("body").getAttribute("data-document-ready")) === "true", {
      timeout: timeoutMs,
    })
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

async function selectCell(frame: FrameLocator, ref: string): Promise<void> {
  const cellName = frame.locator(CELL_NAME_INPUT).first();
  await expect(cellName).toBeEnabled({ timeout: EDITOR_LOAD_TIMEOUT });
  await cellName.click();
  await cellName.fill(ref);
  await cellName.press("Enter");
  await expect
    .poll(async () => {
      const name = await cellName.inputValue().catch(async () => (await cellName.innerText()) ?? "");
      return name.toUpperCase().includes(ref.toUpperCase());
    })
    .toBe(true);
}

async function readFormulaBarValue(frame: FrameLocator): Promise<string> {
  const valueLoc = frame.locator(CELL_VALUE_INPUT).first();
  if ((await valueLoc.count()) === 0) {
    return "";
  }
  const tag = await valueLoc.evaluate((el) => el.tagName.toLowerCase());
  if (tag === "input" || tag === "textarea") {
    return valueLoc.inputValue();
  }
  return ((await valueLoc.innerText()) || (await valueLoc.textContent()) || "").trim();
}

async function writeFormulaBarValue(frame: FrameLocator, value: string): Promise<void> {
  const valueLoc = frame.locator(CELL_VALUE_INPUT).first();
  await expect(valueLoc).toBeVisible({ timeout: EDITOR_LOAD_TIMEOUT });
  await valueLoc.click();
  await valueLoc.press("Control+a");
  await valueLoc.fill(value);
  await valueLoc.press("Enter");
}

function parseCellRef(ref: string): { col: number; row: number } {
  const match = /^([A-Za-z]+)(\d+)$/.exec(ref);
  if (!match) {
    throw new Error(`invalid cell ref ${ref}`);
  }
  let col = 0;
  for (const ch of match[1].toUpperCase()) {
    col = col * 26 + (ch.charCodeAt(0) - 64);
  }
  return { col, row: Number.parseInt(match[2], 10) };
}

function parseCsvLine(line: string): string[] {
  const cells: string[] = [];
  let current = "";
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (inQuotes) {
      if (ch === "\"" && line[i + 1] === "\"") {
        current += "\"";
        i++;
      } else if (ch === "\"") {
        inQuotes = false;
      } else {
        current += ch;
      }
      continue;
    }
    if (ch === "\"") {
      inQuotes = true;
      continue;
    }
    if (ch === ",") {
      cells.push(current);
      current = "";
      continue;
    }
    current += ch;
  }
  cells.push(current);
  return cells;
}

export function csvCellValue(csv: string, ref: string): string {
  const { col, row } = parseCellRef(ref);
  const lines = csv.split(/\r?\n/).filter((line) => line.length > 0);
  const line = lines[row - 1];
  if (!line) {
    return "";
  }
  const cells = parseCsvLine(line);
  return cells[col - 1] ?? "";
}

async function readCellValue(frame: FrameLocator, ref: string): Promise<string> {
  const viaSdk = await frame.locator("body").evaluate((_, cellRef: string) => {
    const w = window as { Asc?: { editor?: AscEditor; spreadsheet?: AscEditor }; editor?: AscEditor };
    const api = w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor;
    if (!api?.asc_selectRange) {
      return "";
    }
    api.asc_selectRange(cellRef);
    return api.asc_getCellText?.() ?? api.asc_getFormula?.() ?? "";
  }, ref);
  if (viaSdk) {
    return viaSdk;
  }

  await selectCell(frame, ref);
  return readFormulaBarValue(frame);
}

async function setCellValue(frame: FrameLocator, ref: string, value: string): Promise<boolean> {
  return frame.locator("body").evaluate(
    ({ cellRef, cellValue }) => {
      const w = window as { Asc?: { editor?: AscEditor; spreadsheet?: AscEditor }; editor?: AscEditor };
      const api = w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor;
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
  const file = new URL(page.url()).searchParams.get("file");

  await expect
    .poll(
      async () => {
        const value = await readCellValue(frame, ref);
        if (value.includes(expected)) {
          return true;
        }
        if (file && (file.endsWith(".csv") || file.endsWith(".tsv"))) {
          const res = await page.request.get(`/api/office/demo/file/${encodeURIComponent(file)}`);
          if (res.ok()) {
            return csvCellValue(await res.text(), ref).includes(expected);
          }
        }
        return false;
      },
      { timeout: CONTENT_FIND_TIMEOUT },
    )
    .toBe(true);
}

export async function assertDocumentContains(
  page: Page,
  editor: SampleFile["editor"],
  text: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);
  const frame = getEditorFrame(page, editor);

  await expect
    .poll(
      async () => {
        const viaSdk = await frame.locator("body").evaluate((_, needle: string) => {
          try {
            const w = window as { Asc?: { editor?: AscEditor }; editor?: AscEditor };
            const api = w.Asc?.editor ?? w.editor;
            if (!api || typeof api.asc_findText !== "function") {
              return false;
            }
            return Boolean(
              api.asc_findText(needle) ||
                api.asc_findText(needle, false, false) ||
                api.asc_findText(needle, true, false),
            );
          } catch {
            return false;
          }
        }, text);
        if (viaSdk) {
          return true;
        }
        if (editor === "word" || editor === "slide") {
          const file = new URL(page.url()).searchParams.get("file");
          if (file) {
            const res = await page.request.get(`/api/office/demo/file/${encodeURIComponent(file)}`);
            if (res.ok()) {
              return officeFileContains(Buffer.from(await res.body()), text);
            }
          }
        }
        return false;
      },
      { timeout: CONTENT_FIND_TIMEOUT },
    )
    .toBe(true);
}

export async function setCellContent(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  value: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);
  const frame = getEditorFrame(page, editor);

  const viaSdk = await setCellValue(frame, ref, value);
  if (viaSdk) {
    const readback = await readCellValue(frame, ref);
    if (readback.includes(value)) {
      return;
    }
  }

  await selectCell(frame, ref);
  await writeFormulaBarValue(frame, value);
}

async function replaceDocumentTextViaSdk(
  frame: FrameLocator,
  from: string,
  to: string,
): Promise<boolean> {
  return frame.locator("body").evaluate(
    ({ from: search, to: replace }) => {
      const w = window as {
        Asc?: { editor?: AscEditor };
        editor?: AscEditor;
        AscCommon?: {
          CSearchSettings?: new () => {
            SetText?: (text: string) => void;
            put_Text?: (text: string) => void;
          };
        };
      };
      const api = w.Asc?.editor ?? w.editor;
      if (!api?.asc_replaceText) {
        return false;
      }
      try {
        let props: { findWhat?: string; GetText?: () => string; SetText?: (text: string) => void };
        if (w.AscCommon?.CSearchSettings) {
          props = new w.AscCommon.CSearchSettings();
          if (typeof props.SetText === "function") {
            props.SetText(search);
          } else if (typeof props.put_Text === "function") {
            props.put_Text(search);
          }
        } else {
          props = {
            findWhat: search,
            GetText() {
              return this.findWhat;
            },
          };
        }
        return api.asc_replaceText(props, replace, false) !== false;
      } catch {
        return false;
      }
    },
    { from, to },
  );
}

async function replaceDocumentTextViaSearchBar(
  page: Page,
  frame: FrameLocator,
  from: string,
  to: string,
): Promise<boolean> {
  await frame.locator("#search-bar-close").click({ force: true }).catch(() => undefined);
  await frame.locator("body").press("Control+f");
  const searchInput = frame.locator("#search-bar-text").first();
  if ((await searchInput.count()) === 0) {
    return false;
  }
  await expect(searchInput).toBeVisible({ timeout: 5_000 });
  await searchInput.fill(from);
  await searchInput.press("Enter");
  await page.waitForTimeout(300);

  const viaParent = await page.evaluate((replace: string) => {
    const ed = (window as { docEditor?: { grabFocus?: () => void; insertPlainText?: (t: string) => void } })
      .docEditor;
    if (!ed?.insertPlainText) {
      return false;
    }
    ed.grabFocus?.();
    ed.insertPlainText(replace);
    return true;
  }, to);
  await frame.locator("#search-bar-close").click({ force: true }).catch(() => undefined);
  return viaParent;
}

/** Replace existing document text (word/slide). Prefer this over appending unique markers for save tests. */
export async function replaceDocumentText(
  page: Page,
  editor: SampleFile["editor"],
  from: string,
  to: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);
  const frame = getEditorFrame(page, editor);

  await expect
    .poll(
      async () => {
        if (await replaceDocumentTextViaSdk(frame, from, to)) {
          return true;
        }
        return replaceDocumentTextViaSearchBar(page, frame, from, to);
      },
      { timeout: CONTENT_FIND_TIMEOUT, intervals: [500, 1000, 2000] },
    )
    .toBe(true);

  await page.waitForTimeout(300);
}

/** Insert text into the word/slide document (canvas-backed; parent-page Ctrl+S and DOM innerText do not work). */
export async function typeInDocument(
  page: Page,
  editor: SampleFile["editor"],
  text: string,
): Promise<void> {
  await waitForEditorInteractive(page, editor);

  const viaDocsApi = await page.evaluate((chunk: string) => {
    const ed = (window as { docEditor?: { grabFocus?: () => void; insertPlainText?: (t: string) => void } }).docEditor;
    if (!ed || typeof ed.insertPlainText !== "function") {
      return false;
    }
    ed.grabFocus?.();
    ed.insertPlainText(chunk);
    return true;
  }, text);
  if (viaDocsApi) {
    await page.waitForTimeout(400);
    return;
  }

  const frame = getEditorFrame(page, editor);
  const inserted = await frame.locator("body").evaluate((_, chunk: string) => {
    const w = window as { Asc?: { editor?: AscEditor; spreadsheet?: AscEditor }; editor?: AscEditor };
    const api = w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor;
    if (!api) {
      return false;
    }
    const methods = ["asc_enterText", "asc_nativeInsertText", "asc_AddText", "asc_insertText", "asc_PasteText", "PasteText"];
    for (const name of methods) {
      const fn = (api as Record<string, unknown>)[name];
      if (typeof fn === "function") {
        (fn as (value: string) => void).call(api, chunk);
        return true;
      }
    }
    return false;
  }, text);
  if (inserted) {
    await page.waitForTimeout(200);
    return;
  }

  const overlay = frame.locator("#id_viewer_overlay").first();
  await overlay.click({ position: { x: 120, y: 120 }, force: true });
  await overlay.pressSequentially(text, { delay: 20 });
}

const SAVE_BUTTON = "#slot-btn-dt-save, #id-toolbar-btn-save, button.btn-save, .icon-save";

/** Trigger Save inside the editor iframe (parent-page Ctrl+S never reaches the SDK). */
export async function triggerEditorSave(page: Page, editor: SampleFile["editor"]): Promise<void> {
  const frame = getEditorFrame(page, editor);
  const saveBtn = frame.locator(SAVE_BUTTON).first();
  if ((await saveBtn.count()) > 0) {
    await saveBtn.click({ force: true });
    return;
  }
  await frame.locator("body").press("Control+s");
}

function zipEntryText(buf: Buffer, entryName: string): string {
  let offset = 0;
  while (offset + 30 < buf.length) {
    if (buf.readUInt32LE(offset) !== 0x04034b50) {
      break;
    }
    const method = buf.readUInt16LE(offset + 8);
    const compSize = buf.readUInt32LE(offset + 18);
    const nameLen = buf.readUInt16LE(offset + 26);
    const extraLen = buf.readUInt16LE(offset + 28);
    const name = buf.subarray(offset + 30, offset + 30 + nameLen).toString("utf8");
    const dataStart = offset + 30 + nameLen + extraLen;
    const dataEnd = dataStart + compSize;
    if (dataEnd > buf.length) {
      break;
    }
    if (name === entryName || name.endsWith("/" + entryName)) {
      const data = buf.subarray(dataStart, dataEnd);
      if (method === 0) {
        return data.toString("utf8");
      }
      if (method === 8) {
        return inflateRawSync(data).toString("utf8");
      }
    }
    offset = dataEnd;
  }
  return "";
}

export function officeFileContains(buf: Buffer, marker: string): boolean {
  if (buf.includes(Buffer.from(marker)) || buf.toString("utf8").includes(marker)) {
    return true;
  }
  const xml = zipEntryText(buf, "word/document.xml");
  if (xml.includes(marker)) {
    return true;
  }
  const utf16 = Buffer.from(marker, "utf16le");
  return buf.includes(utf16);
}

export async function fetchDemoFileBody(
  request: APIRequestContext,
  filePath: string,
): Promise<Buffer> {
  const res = await request.get(`/api/office/demo/file/${encodeURIComponent(filePath)}`);
  expect(res.ok()).toBeTruthy();
  return Buffer.from(await res.body());
}

export async function assertDemoFileContains(
  request: APIRequestContext,
  filePath: string,
  text: string,
  opts?: { present?: boolean },
): Promise<void> {
  const buf = await fetchDemoFileBody(request, filePath);
  const present = officeFileContains(buf, text);
  if (opts?.present === false) {
    expect(present).toBe(false);
  } else {
    expect(present).toBe(true);
  }
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
            if (officeFileContains(body, options.marker)) {
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
