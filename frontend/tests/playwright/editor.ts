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

const EDITOR_LOAD_TIMEOUT = Number(process.env.PLAYWRIGHT_EDITOR_TIMEOUT ?? 45_000);
const DOCUMENT_READY_TIMEOUT = Number(process.env.PLAYWRIGHT_DOCUMENT_READY_TIMEOUT ?? 45_000);
const DEMO_WARM_TIMEOUT = Number(process.env.PLAYWRIGHT_WARM_TIMEOUT ?? 90_000);
const CONTENT_FIND_TIMEOUT = Number(process.env.PLAYWRIGHT_CONTENT_FIND_TIMEOUT ?? 15_000);
const SAVE_DONE_TIMEOUT = Number(process.env.PLAYWRIGHT_SAVE_DONE_TIMEOUT ?? 120_000);
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
  asc_findText?: (
    text: string | { searchString: string; matchCase?: boolean },
    matchCase?: boolean,
    wholeCell?: boolean,
  ) => boolean;
  asc_replaceText?: (
    searchProps: { searchString: string; matchCase?: boolean } | string,
    replaceWith: string,
    replaceAll?: boolean,
  ) => boolean | void;
  asc_isDocumentCanSave?: () => boolean;
  asc_setFontBold?: (value: boolean) => void;
  asc_setFontItalic?: (value: boolean) => void;
  asc_putHighlight?: (color: string) => void;
  asc_putHighlightColor?: (color: string) => void;
};

type AscEditorWindow = {
  Asc?: { editor?: AscEditor; spreadsheet?: AscEditor; presentation?: AscEditor };
  editor?: AscEditor;
};

function cellApiReadyInBrowser(): boolean {
  const w = window as AscEditorWindow;
  const api = w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor;
  if (!api) {
    return false;
  }
  return (
    typeof api.asc_selectRange === "function" ||
    typeof api.asc_setCellValue === "function" ||
    typeof api.asc_insertText === "function"
  );
}

function cellSetApiReadyInBrowser(): boolean {
  const w = window as AscEditorWindow;
  const api = w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor;
  if (!api) {
    return false;
  }
  const canSelect = typeof api.asc_selectRange === "function";
  const canWrite =
    typeof api.asc_setCellValue === "function" || typeof api.asc_insertText === "function";
  return canSelect && canWrite;
}

function selectCellInBrowser(cellRef: string): boolean {
  const w = window as AscEditorWindow;
  const api = w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor;
  if (!api?.asc_selectRange) {
    return false;
  }
  try {
    api.asc_selectRange(cellRef);
    return true;
  } catch {
    return false;
  }
}

function wordSlideEditableInBrowser(kind: SampleFile["editor"]): boolean {
  const w = window as AscEditorWindow & {
    docEditor?: { insertPlainText?: (t: string) => void };
  };
  if (typeof w.docEditor?.insertPlainText === "function") {
    return true;
  }
  const api =
    kind === "slide"
      ? w.Asc?.presentation ?? w.Asc?.editor ?? w.editor
      : kind === "word"
        ? w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor
        : w.Asc?.editor ?? w.Asc?.presentation ?? w.Asc?.spreadsheet ?? w.editor;
  if (typeof api?.asc_insertText === "function") {
    return true;
  }
  return typeof api?.asc_AddText === "function" || typeof api?.asc_enterText === "function";
}

function wordSlideInteractiveInBrowser(kind: SampleFile["editor"]): boolean {
  const w = window as AscEditorWindow;
  const api =
    kind === "slide"
      ? w.Asc?.presentation ?? w.Asc?.editor ?? w.editor
      : kind === "word"
        ? w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor
        : w.Asc?.editor ?? w.Asc?.presentation ?? w.Asc?.spreadsheet ?? w.editor;
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
}

function readCellViaBrowser(cellRef: string): string {
  const w = window as AscEditorWindow;
  const api = w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor;
  if (!api?.asc_selectRange) {
    return "";
  }
  api.asc_selectRange(cellRef);
  return api.asc_getCellText?.() ?? api.asc_getFormula?.() ?? "";
}

function setCellViaBrowser(arg: { cellRef: string; cellValue: string }): boolean {
  const w = window as AscEditorWindow;
  const api = w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor;
  if (!api) {
    return false;
  }
  if (typeof api.asc_selectRange === "function") {
    api.asc_selectRange(arg.cellRef);
  }
  if (typeof api.asc_setCellValue === "function") {
    api.asc_setCellValue(arg.cellValue);
    api.asc_closeCellEditor?.(true);
    return true;
  }
  if (typeof api.asc_insertText === "function") {
    api.asc_insertText(arg.cellValue);
    api.asc_closeCellEditor?.(true);
    return true;
  }
  return false;
}

function findTextInBrowser(arg: { needle: string; kind?: SampleFile["editor"] }): boolean {
  const w = window as AscEditorWindow;
  const api =
    arg.kind === "cell"
      ? w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor
      : arg.kind === "slide"
        ? w.Asc?.presentation ?? w.Asc?.editor ?? w.editor
        : arg.kind === "word"
          ? w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor
          : w.Asc?.editor ?? w.Asc?.presentation ?? w.Asc?.spreadsheet ?? w.editor;
  if (!api || typeof api.asc_findText !== "function") {
    return false;
  }
  const props = { searchString: arg.needle, matchCase: false };
  try {
    return Boolean(
      api.asc_findText(props) ||
        api.asc_findText(props, false, false) ||
        api.asc_findText(arg.needle, false, false),
    );
  } catch {
    return false;
  }
}

function replaceTextInBrowser(arg: { from: string; to: string; kind?: SampleFile["editor"] }): boolean {
  const w = window as AscEditorWindow;
  const api =
    arg.kind === "cell"
      ? w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor
      : arg.kind === "slide"
        ? w.Asc?.presentation ?? w.Asc?.editor ?? w.editor
        : arg.kind === "word"
          ? w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor
          : w.Asc?.editor ?? w.Asc?.presentation ?? w.Asc?.spreadsheet ?? w.editor;
  if (!api?.asc_replaceText) {
    return false;
  }
  const searchProps = { searchString: arg.from, matchCase: false };
  try {
    api.asc_replaceText(searchProps, arg.to, true);
    const toProps = { searchString: arg.to, matchCase: false };
    return Boolean(
      api.asc_findText?.(toProps) ||
        api.asc_findText?.(toProps, false, false) ||
        api.asc_findText?.(arg.to, false, false),
    );
  } catch {
    return false;
  }
}

function insertTextInBrowser(arg: { chunk: string; kind: SampleFile["editor"] }): boolean {
  const w = window as AscEditorWindow;
  const api =
    arg.kind === "cell"
      ? w.Asc?.spreadsheet ?? w.Asc?.editor ?? w.editor
      : arg.kind === "slide"
        ? w.Asc?.presentation ?? w.Asc?.editor ?? w.editor
        : arg.kind === "word"
          ? w.Asc?.editor ?? w.Asc?.spreadsheet ?? w.editor
          : w.Asc?.editor ?? w.Asc?.presentation ?? w.Asc?.spreadsheet ?? w.editor;
  if (!api) {
    return false;
  }
  const methods = ["asc_enterText", "asc_nativeInsertText", "asc_AddText", "asc_insertText", "asc_PasteText", "PasteText"];
  for (const name of methods) {
    const fn = (api as Record<string, unknown>)[name];
    if (typeof fn === "function") {
      (fn as (value: string) => void).call(api, arg.chunk);
      return true;
    }
  }
  return false;
}

export function getEditorFrame(page: Page, editor: SampleFile["editor"]): FrameLocator {
  const app = EDITOR_APP[editor];
  return page.frameLocator(`iframe[src*="/${app}/"]`).first();
}

async function isLoadMaskBlocking(frame: FrameLocator): Promise<boolean> {
  try {
    return await frame.locator("body").evaluate((_, selectors: string[]) => {
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
  } catch {
    return false;
  }
}

async function isEditorShellReady(
  frame: FrameLocator,
  editor: SampleFile["editor"],
): Promise<boolean> {
  const probeTimeout = 3_000;
  try {
    if (editor === "pdf") {
      return await frame.locator("#id_view, #id_main").evaluateAll(
        (nodes) =>
          nodes.some((node) => {
            const rect = node.getBoundingClientRect();
            return rect.width > 50 && rect.height > 50;
          }),
        { timeout: probeTimeout },
      );
    }

    return await frame.locator(EDITOR_SHELL).evaluateAll(
      (nodes) =>
        nodes.some((node) => {
          const rect = node.getBoundingClientRect();
          return rect.width > 50 && rect.height > 50;
        }),
      { timeout: probeTimeout },
    );
  } catch {
    return false;
  }
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
    const formulaBar = frame.locator(CELL_VALUE_INPUT).first();
    if ((await cellName.count()) === 0 || (await formulaBar.count()) === 0) {
      return false;
    }
    const shellReady = await isEditorShellReady(frame, editor);
    if (!shellReady) {
      return false;
    }
    if (!(await formulaBar.isVisible().catch(() => false))) {
      return false;
    }
    if (await frame.locator("body").evaluate(cellApiReadyInBrowser)) {
      return true;
    }
    return cellName.isEnabled().catch(() => false);
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
    return frame.locator("body").evaluate(wordSlideInteractiveInBrowser, editor);
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

async function isEditorEditable(
  page: Page,
  frame: FrameLocator,
  editor: SampleFile["editor"],
): Promise<boolean> {
  if ((await page.locator("body").getAttribute("data-content-ready")) !== "true") {
    return false;
  }
  if (await isLoadMaskBlocking(frame)) {
    return false;
  }
  if (editor === "cell") {
    return frame.locator("body").evaluate(cellSetApiReadyInBrowser).catch(() => false);
  }
  if (editor === "word" || editor === "slide") {
    const parentInsertReady = await page.evaluate(() => {
      const ed = (window as { docEditor?: { insertPlainText?: (t: string) => void } }).docEditor;
      return typeof ed?.insertPlainText === "function";
    });
    if (parentInsertReady) {
      return true;
    }
    return frame.locator("body").evaluate(wordSlideEditableInBrowser, editor).catch(() => false);
  }
  return isEditorInteractive(page, frame, editor);
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

/** Demo viewer finished synchronous warm (queue time not counted toward editor-ready timeout). */
export async function waitForDemoWarm(page: Page, timeoutMs = DEMO_WARM_TIMEOUT): Promise<void> {
  await page.waitForFunction(
    () => document.body.getAttribute("data-warm-done") === "true",
    { timeout: timeoutMs },
  );
}

/** Block until x2t has produced Editor.bin for file (use before goto for forked save paths). */
export async function warmDemoFile(
  request: APIRequestContext,
  filePath: string,
): Promise<void> {
  const warm = await request.get(`/demo/warm?file=${encodeURIComponent(filePath)}`);
  if (!warm.ok()) {
    throw new Error(`warm ${filePath}: HTTP ${warm.status()} ${await warm.text()}`);
  }
}

/** Editor iframe mounted and document is ready to use (open-format tests). */
export async function waitForEditorReady(
  page: Page,
  editor: SampleFile["editor"],
): Promise<void> {
  try {
    await waitForDemoWarm(page);
    await page.waitForFunction(
      () => {
        const status = (document.getElementById("status")?.textContent ?? "").trim();
        if (status.startsWith("Error:")) {
          throw new Error(`viewer status: ${status}`);
        }
        return (
          document.body.getAttribute("data-document-ready") === "true" ||
          status === "Document ready"
        );
      },
      { timeout: DOCUMENT_READY_TIMEOUT },
    );
    const app = EDITOR_APP[editor];
    await expect
      .poll(
        async () => {
          if ((await page.locator(`iframe[src*="/${app}/"]`).count()) === 0) {
            return false;
          }
          return isEditorShellReady(getEditorFrame(page, editor), editor);
        },
        { timeout: DOCUMENT_READY_TIMEOUT },
      )
      .toBe(true);
  } catch (err) {
    throw new Error(`${String(err)}\nwitness:\n${await editorWitness(page, editor)}`);
  }
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
  if (editor === "cell") {
    await dismissEditorOverlays(frame);
  }
}

/** Document content is editable (onDocumentContentReady + bound editor APIs). Use for save tests. */
export async function waitForEditorEditable(
  page: Page,
  editor: SampleFile["editor"],
  timeoutMs = DOCUMENT_READY_TIMEOUT,
): Promise<void> {
  try {
    await waitForEditorReady(page, editor);
    const frame = getEditorFrame(page, editor);
    await expect
      .poll(async () => isEditorEditable(page, frame, editor), { timeout: timeoutMs })
      .toBe(true);
    await page.waitForTimeout(INTERACTIVE_SETTLE_MS);
    if (editor === "cell") {
      await dismissEditorOverlays(frame);
    }
  } catch (err) {
    throw new Error(`${String(err)}\nwitness:\n${await editorWitness(page, editor)}`);
  }
}

async function dismissEditorOverlays(frame: FrameLocator): Promise<void> {
  const tip = frame.locator(".synch-tip-root, .asc-synchronizetip").first();
  if ((await tip.count()) === 0) {
    return;
  }
  const closeBtn = tip.locator(
    'button, .close, [id*="close"], .btn-close, .asc-synchronizetip-close, .tip-close',
  ).first();
  if ((await closeBtn.count()) > 0) {
    await closeBtn.click({ timeout: 1_000 }).catch(() => {});
  } else {
    await frame.locator("body").press("Escape").catch(() => {});
  }
  await expect(tip).toBeHidden({ timeout: 3_000 }).catch(() => {});
}

/** @deprecated Use waitForEditorReady — body data-document-ready is unreliable under load. */
export async function waitForDocumentReady(
  page: Page,
  editor: SampleFile["editor"],
): Promise<void> {
  await waitForEditorReady(page, editor);
  await waitForEditorInteractive(page, editor);
}

async function cellNameShowsRef(frame: FrameLocator, ref: string): Promise<boolean> {
  const cellName = frame.locator(CELL_NAME_INPUT).first();
  const name = await cellName.inputValue().catch(async () => (await cellName.innerText()) ?? "");
  return name.toUpperCase().includes(ref.toUpperCase());
}

async function selectCellViaSdk(frame: FrameLocator, ref: string): Promise<boolean> {
  const selected = await frame
    .locator("body")
    .evaluate(selectCellInBrowser, ref)
    .catch(() => false);
  if (!selected) {
    return false;
  }
  return cellNameShowsRef(frame, ref);
}

async function selectCellViaUi(frame: FrameLocator, ref: string): Promise<boolean> {
  const cellName = frame.locator(CELL_NAME_INPUT).first();
  if (!(await cellName.isEnabled().catch(() => false))) {
    return false;
  }
  await dismissEditorOverlays(frame);
  await cellName.click({ timeout: 2_000 }).catch(() => {});
  await cellName.fill(ref);
  await cellName.press("Enter");
  await settleFrame(frame, 200);
  return cellNameShowsRef(frame, ref);
}

/** Select a spreadsheet cell via SDK when available, otherwise the name box UI. */
async function selectCell(frame: FrameLocator, ref: string): Promise<void> {
  const cellName = frame.locator(CELL_NAME_INPUT).first();
  await expect(cellName).toBeEnabled({ timeout: EDITOR_LOAD_TIMEOUT });
  await expect
    .poll(
      async () => {
        if (await isLoadMaskBlocking(frame)) {
          return false;
        }
        if (await selectCellViaSdk(frame, ref)) {
          return true;
        }
        return selectCellViaUi(frame, ref);
      },
      { timeout: EDITOR_LOAD_TIMEOUT, intervals: [200, 500, 1000] },
    )
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
  await dismissEditorOverlays(frame);
  try {
    await valueLoc.click({ timeout: 3_000 });
  } catch {
    await dismissEditorOverlays(frame);
    await valueLoc.click({ force: true });
  }
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
  const viaSdk = await frame.locator("body").evaluate(readCellViaBrowser, ref);
  if (viaSdk) {
    return viaSdk;
  }

  await selectCell(frame, ref);
  return readFormulaBarValue(frame);
}

async function setCellValue(frame: FrameLocator, ref: string, value: string): Promise<boolean> {
  return frame.locator("body").evaluate(setCellViaBrowser, { cellRef: ref, cellValue: value });
}

async function cellShowsValue(
  page: Page,
  frame: FrameLocator,
  editor: SampleFile["editor"],
  ref: string,
  expected: string,
): Promise<boolean> {
  const file = new URL(page.url()).searchParams.get("file");
  if (file && (file.endsWith(".csv") || file.endsWith(".tsv"))) {
    const res = await page.request.get(`/api/office/demo/file/${encodeURIComponent(file)}`, {
      headers: { "Cache-Control": "no-cache" },
    });
    if (res.ok() && csvCellValue(await res.text(), ref).includes(expected)) {
      return true;
    }
  }

  try {
    const viaSdk = await frame.locator("body").evaluate(readCellViaBrowser, ref);
    if (viaSdk.includes(expected)) {
      return true;
    }
  } catch {
    // fall through to UI readback
  }

  if (!(await isEditorInteractive(page, frame, editor))) {
    return false;
  }
  try {
    const value = await readCellValue(frame, ref);
    return value.includes(expected);
  } catch {
    return false;
  }
}

/** Poll until a spreadsheet cell shows expected text in the editor (or CSV on disk). */
export async function waitForCellContent(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  expected: string,
  timeoutMs = EDITOR_LOAD_TIMEOUT,
): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await expect
    .poll(() => cellShowsValue(page, frame, editor, ref, expected), {
      timeout: timeoutMs,
      intervals: [200, 500, 1000],
    })
    .toBe(true);
}

export async function assertCellContent(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  expected: string,
): Promise<void> {
  await waitForEditorReady(page, editor);
  await waitForCellContent(page, editor, ref, expected, CONTENT_FIND_TIMEOUT);
}

export async function assertDocumentContains(
  page: Page,
  editor: SampleFile["editor"],
  text: string,
): Promise<void> {
  await waitForEditorReady(page, editor);
  const frame = getEditorFrame(page, editor);
  const file = new URL(page.url()).searchParams.get("file");

  await expect
    .poll(
      async () => {
        if (file && (editor === "word" || editor === "slide")) {
          const res = await page.request.get(`/api/office/demo/file/${encodeURIComponent(file)}`);
          if (res.ok() && officeFileContains(Buffer.from(await res.body()), text)) {
            return true;
          }
        }
        if (!(await isEditorInteractive(page, frame, editor))) {
          return false;
        }
        try {
          return await frame.locator("body").evaluate(findTextInBrowser, { needle: text, kind: editor });
        } catch {
          return false;
        }
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
  await dismissEditorOverlays(frame);

  let viaSdk = false;
  try {
    await expect
      .poll(
        async () => {
          if (await isLoadMaskBlocking(frame)) {
            return false;
          }
          const apiReady = await frame
            .locator("body")
            .evaluate(cellSetApiReadyInBrowser)
            .catch(() => false);
          if (!apiReady) {
            return false;
          }
          return setCellValue(frame, ref, value);
        },
        { timeout: EDITOR_LOAD_TIMEOUT, intervals: [200, 500, 1000] },
      )
      .toBe(true);
    viaSdk = true;
  } catch {
    viaSdk = false;
  }

  if (!viaSdk) {
    await selectCell(frame, ref);
    await writeFormulaBarValue(frame, value);
  }

  await commitCellEdit(frame);
  if (!(await cellShowsValue(page, frame, editor, ref, value))) {
    await selectCell(frame, ref);
    await writeFormulaBarValue(frame, value);
    await commitCellEdit(frame);
  }
  await settleFrame(frame, 500);
}

/** Set a cell value for save tests after content-ready (no readback poll loop). */
export async function editCellForSave(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  value: string,
): Promise<void> {
  await waitForEditorEditable(page, editor);
  const frame = getEditorFrame(page, editor);
  await dismissEditorOverlays(frame);

  const viaSdk = await setCellValue(frame, ref, value).catch(() => false);
  if (!viaSdk) {
    await selectCell(frame, ref);
    await writeFormulaBarValue(frame, value);
  }
  await commitCellEdit(frame);
  await settleFrame(frame, 300);
}

async function commitCellEdit(frame: FrameLocator): Promise<void> {
  const valueLoc = frame.locator(CELL_VALUE_INPUT).first();
  if ((await valueLoc.count()) > 0) {
    await valueLoc.press("Enter").catch(() => {});
  }
  const cellName = frame.locator(CELL_NAME_INPUT).first();
  if ((await cellName.count()) > 0) {
    await cellName.click({ timeout: 3_000 }).catch(() => {});
  }
  await settleFrame(frame, 250);
}

async function findDocumentTextViaSdk(
  frame: FrameLocator,
  text: string,
  editor?: SampleFile["editor"],
): Promise<boolean> {
  return frame.locator("body").evaluate(findTextInBrowser, { needle: text, kind: editor });
}

async function replaceDocumentTextViaSdk(
  frame: FrameLocator,
  from: string,
  to: string,
  editor?: SampleFile["editor"],
): Promise<boolean> {
  return frame.locator("body").evaluate(replaceTextInBrowser, { from, to, kind: editor });
}

async function settleFrame(frame: FrameLocator, ms = 350): Promise<void> {
  await frame.locator("body").evaluate((delay) => new Promise((r) => setTimeout(r, delay)), ms);
}

async function closeSearchBar(frame: FrameLocator): Promise<void> {
  await frame.locator("#search-bar-close, #search-adv-close").first().click({ timeout: 1_000 }).catch(() => {});
}

async function searchBarHasMatches(frame: FrameLocator): Promise<boolean> {
  const results = frame.locator("#search-bar-results").first();
  const text = (await results.textContent({ timeout: 1_000 }).catch(() => "")) ?? "";
  const match = text.trim().match(/^(\d+)\s*\/\s*(\d+)$/);
  if (!match) {
    return false;
  }
  const total = Number.parseInt(match[2], 10);
  return total > 0;
}

async function findDocumentTextViaSearchUI(frame: FrameLocator, text: string): Promise<boolean> {
  try {
    await frame.locator("body").click({ position: { x: 8, y: 8 }, force: true }).catch(() => {});
    await frame.locator("body").press("Control+f");
    const searchInput = frame.locator("#search-bar-text").first();
    await searchInput.waitFor({ state: "visible", timeout: 4_000 });
    await searchInput.fill(text);
    await searchInput.press("Enter");
    await settleFrame(frame, 500);
    const hasMatch = await searchBarHasMatches(frame);
    await closeSearchBar(frame);
    return hasMatch;
  } catch {
    await closeSearchBar(frame);
    return false;
  }
}

async function documentContainsText(
  frame: FrameLocator,
  text: string,
  editor: SampleFile["editor"],
): Promise<boolean> {
  if (await findDocumentTextViaSdk(frame, text, editor)) {
    return true;
  }
  return await findDocumentTextViaSearchUI(frame, text);
}

async function replaceDocumentTextViaSearchUI(
  frame: FrameLocator,
  from: string,
  to: string,
  editor: SampleFile["editor"],
): Promise<boolean> {
  try {
    await frame.locator("body").click({ position: { x: 10, y: 10 }, force: true }).catch(() => {});
    await frame.locator("body").press("Control+f");
    const searchInput = frame.locator("#search-bar-text").first();
    await searchInput.waitFor({ state: "visible", timeout: 8_000 });
    await searchInput.fill(from);
    await searchInput.press("Enter");
    await settleFrame(frame, 500);
    if (!(await searchBarHasMatches(frame))) {
      await closeSearchBar(frame);
      return false;
    }

    const openPanel = frame.locator("#search-bar-open-panel, #search-bar-open-panel-redact").first();
    if (await openPanel.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await openPanel.click();
      await settleFrame(frame, 300);
    }
    const replaceInput = frame
      .locator(
        "#search-adv-replace-input input, #search-adv-replace-text input, #search-adv-replace-input, #search-adv-replace-text, input[placeholder*='Replace' i]",
      )
      .first();
    if (await replaceInput.isVisible({ timeout: 3_000 }).catch(() => false)) {
      await replaceInput.fill(to);
      const replaceAll = frame.locator("#search-adv-replace-all").first();
      if (await replaceAll.isVisible({ timeout: 1_000 }).catch(() => false)) {
        await replaceAll.click();
      } else {
        await frame.locator("#search-adv-replace").first().click();
      }
    } else {
      await frame.locator("body").pressSequentially(to, { delay: 20 });
    }

    await settleFrame(frame, 500);
    await closeSearchBar(frame);
    return await documentContainsText(frame, to, editor);
  } catch {
    await closeSearchBar(frame);
    return false;
  }
}

async function isDocumentDirty(page: Page): Promise<boolean> {
  return (await page.locator("body").getAttribute("data-dirty")) === "true";
}

/** Wait until the demo viewer has no pending unsaved edits. */
export async function waitForDocumentClean(page: Page, timeoutMs = 15_000): Promise<void> {
  await expect
    .poll(async () => !(await isDocumentDirty(page)), { timeout: timeoutMs })
    .toBe(true);
}

/** Wait until the demo viewer marks the document as having unsaved edits. */
export async function waitForDocumentDirty(page: Page, timeoutMs = 10_000): Promise<void> {
  await expect
    .poll(async () => isDocumentDirty(page), { timeout: timeoutMs })
    .toBe(true);
}

/** Poll until marker text is visible in the editor (not on disk). */
async function waitForMarkerInEditor(
  page: Page,
  editor: SampleFile["editor"],
  marker: string,
  timeoutMs = EDITOR_LOAD_TIMEOUT,
): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await expect
    .poll(async () => documentContainsText(frame, marker, editor), {
      timeout: timeoutMs,
      intervals: [300, 500, 1000],
    })
    .toBe(true);
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
    .poll(async () => documentContainsText(frame, from, editor), {
      timeout: CONTENT_FIND_TIMEOUT,
      intervals: [500, 1000, 2000],
    })
    .toBe(true);

  await expect
    .poll(
      async () => {
        const hasTo = await documentContainsText(frame, to, editor);
        const hasFrom = await documentContainsText(frame, from, editor);
        if (hasTo && !hasFrom) {
          if (!(await isDocumentDirty(page))) {
            await frame.locator("body").evaluate(() => {
              const w = window as { Asc?: { editor?: { asc_insertText?: (t: string) => void } } };
              w.Asc?.editor?.asc_insertText?.(" ");
            }).catch(() => {});
            await page.waitForTimeout(200);
          }
          return true;
        }
        if (await replaceDocumentTextViaSdk(frame, from, to, editor)) {
          await settleFrame(frame, 400);
        } else {
          await replaceDocumentTextViaSearchUI(frame, from, to, editor);
        }
        return false;
      },
      {
        timeout: CONTENT_FIND_TIMEOUT,
        intervals: [500, 1000, 2000],
      },
    )
    .toBe(true);

  await page.waitForTimeout(300);
}

async function insertTextAfterEditable(
  page: Page,
  editor: SampleFile["editor"],
  text: string,
): Promise<void> {
  const viaDocsApi = await page.evaluate((chunk: string) => {
    const ed = (window as { docEditor?: { grabFocus?: () => void; insertPlainText?: (t: string) => void } })
      .docEditor;
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
  const inserted = await frame.locator("body").evaluate(insertTextInBrowser, { chunk: text, kind: editor });
  if (inserted) {
    await page.waitForTimeout(200);
    return;
  }

  const overlay = frame.locator("#id_viewer_overlay").first();
  await overlay.click({ position: { x: 120, y: 120 }, force: true });
  await overlay.pressSequentially(text, { delay: 20 });
}

/** Insert a unique marker into word/slide documents for save tests. */
export async function editWordForSave(
  page: Page,
  editor: SampleFile["editor"],
  marker: string,
): Promise<void> {
  await waitForEditorEditable(page, editor);
  const chunk = ` ${marker}`;
  await insertTextAfterEditable(page, editor, chunk);
  await waitForMarkerInEditor(page, editor, marker);
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
  const inserted = await frame.locator("body").evaluate(insertTextInBrowser, { chunk: text, kind: editor });
  if (inserted) {
    await page.waitForTimeout(200);
    return;
  }

  const overlay = frame.locator("#id_viewer_overlay").first();
  await overlay.click({ position: { x: 120, y: 120 }, force: true });
  await overlay.pressSequentially(text, { delay: 20 });
}

const SAVE_BUTTON =
  "#slot-btn-dt-save, #id-toolbar-btn-save, #box-document-title .btn-save, button.btn-save, a.btn-save, .icon-save";

/** Trigger Save inside the editor iframe (parent-page Ctrl+S never reaches the SDK). */
export async function triggerEditorSave(page: Page, editor: SampleFile["editor"]): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await frame.locator("body").click({ position: { x: 12, y: 12 }, force: true });

  const saveBtn = frame.locator(SAVE_BUTTON).first();
  if ((await saveBtn.count()) > 0) {
    await saveBtn.click({ force: true, timeout: 5_000, noWaitAfter: true }).catch(() => {});
  }
  await frame.locator("body").press("Control+s");

  await expect
    .poll(async () => (await page.locator("body").getAttribute("data-saving")) !== "true", {
      timeout: 30_000,
      intervals: [200, 500, 1000],
    })
    .toBe(true);
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
  const text = decodeOfficeText(buf);
  if (text.includes(marker)) {
    return true;
  }
  if (buf.includes(Buffer.from(marker))) {
    return true;
  }
  const plainRtf = rtfPlainText(text);
  if (plainRtf.includes(marker)) {
    return true;
  }
  for (const entry of [
    "word/document.xml",
    "ppt/slides/slide1.xml",
    "content.xml",
    "xl/sharedStrings.xml",
    "xl/worksheets/sheet1.xml",
  ]) {
    const xml = zipEntryText(buf, entry);
    if (xml.includes(marker)) {
      return true;
    }
  }
  const utf16 = Buffer.from(marker, "utf16le");
  return buf.includes(utf16);
}

/** First short slide title/body string from a presentation file (pptx or ppt after save). */
export function discoverSlideMarker(buf: Buffer): string {
  const slide = zipEntryText(buf, "ppt/slides/slide1.xml");
  const fromXml = slide.match(/<a:t>([^<]{3,60})<\/a:t>/);
  if (fromXml) {
    return fromXml[1];
  }
  const text = decodeOfficeText(buf);
  const words = text.match(/[A-Za-z][A-Za-z0-9 ,.'-]{4,40}/g);
  if (words?.length) {
    return words[0].trim();
  }
  throw new Error("no slide marker found in presentation sample");
}

function decodeOfficeText(buf: Buffer): string {
  if (buf.length === 0) {
    return "";
  }
  let text = buf.toString("utf8");
  if (text.charCodeAt(0) === 0xfeff) {
    text = text.slice(1);
  }
  return text;
}

/** Decode x2t RTF plain text, including \\uc1\\uNN* unicode runs. */
export function rtfPlainText(rtf: string): string {
  let out = "";
  let i = 0;
  let ucSkip = 1;
  while (i < rtf.length) {
    if (i + 1 < rtf.length && rtf[i] === "\\") {
      if (i + 3 < rtf.length && rtf[i + 1] === "'") {
        const hex = rtf.slice(i + 2, i + 4);
        const byte = Number.parseInt(hex, 16);
        if (!Number.isNaN(byte)) {
          out += String.fromCharCode(byte);
          i += 4;
          continue;
        }
      }
      if (i + 3 < rtf.length && rtf[i + 1] === "p" && rtf[i + 2] === "a" && rtf[i + 3] === "r") {
        out += "\n";
        i += 4;
        if (i < rtf.length && rtf[i] === " ") {
          i++;
        }
        continue;
      }
      if (i + 2 < rtf.length && rtf[i + 1] === "u" && rtf[i + 2] === "c") {
        let j = i + 3;
        const start = j;
        while (j < rtf.length && rtf[j] >= "0" && rtf[j] <= "9") {
          j++;
        }
        if (j > start) {
          ucSkip = Number.parseInt(rtf.slice(start, j), 10);
          if (j < rtf.length && rtf[j] === " ") {
            j++;
          }
          i = j;
          continue;
        }
      }
      if (rtf[i + 1] === "u") {
        let j = i + 2;
        let neg = false;
        if (j < rtf.length && rtf[j] === "-") {
          neg = true;
          j++;
        }
        const start = j;
        while (j < rtf.length && rtf[j] >= "0" && rtf[j] <= "9") {
          j++;
        }
        if (j > start) {
          let code = Number.parseInt(rtf.slice(start, j), 10);
          if (neg) {
            code = 65536 + code;
          }
          out += String.fromCharCode(code);
          if (j < rtf.length && rtf[j] === "?") {
            j++;
          }
          for (let skipped = 0; skipped < ucSkip && j < rtf.length; skipped++) {
            j++;
          }
          i = j;
          continue;
        }
      }
      let j = i + 1;
      if (j < rtf.length && rtf[j] === "*") {
        j++;
      }
      while (j < rtf.length && /[A-Za-z]/.test(rtf[j])) {
        j++;
      }
      if (j < rtf.length && rtf[j] === "-") {
        j++;
        while (j < rtf.length && rtf[j] >= "0" && rtf[j] <= "9") {
          j++;
        }
      } else if (j < rtf.length && rtf[j] >= "0" && rtf[j] <= "9") {
        while (j < rtf.length && rtf[j] >= "0" && rtf[j] <= "9") {
          j++;
        }
      }
      if (j < rtf.length && rtf[j] === " ") {
        j++;
      }
      i = j;
      continue;
    }
    if (rtf[i] === "{" || rtf[i] === "}") {
      i++;
      continue;
    }
    out += rtf[i];
    i++;
  }
  return out;
}

function rtfWindowAroundMarker(rtf: string, marker: string, radius = 2500): string {
  const idx = rtf.indexOf(marker);
  if (idx >= 0) {
    const start = Math.max(0, idx - radius);
    const end = Math.min(rtf.length, idx + marker.length + radius);
    return rtf.slice(start, end);
  }
  const plain = rtfPlainText(rtf);
  const plainIdx = plain.indexOf(marker);
  if (plainIdx < 0) {
    return "";
  }
  const ratio = plain.length > 0 ? rtf.length / plain.length : 1;
  const center = Math.floor(plainIdx * ratio);
  const start = Math.max(0, center - radius);
  const end = Math.min(rtf.length, center + marker.length + radius);
  return rtf.slice(start, end);
}

export type WordFormatOptions = {
  bold?: boolean;
  italic?: boolean;
  highlight?: boolean;
};

function applyWordFormatInBrowser(arg: { marker: string; format: WordFormatOptions }): boolean {
  const w = window as AscEditorWindow;
  const api = w.Asc?.editor ?? w.editor;
  if (!api || typeof api.asc_findText !== "function") {
    return false;
  }
  const props = { searchString: arg.marker, matchCase: false };
  let found = false;
  try {
    found = Boolean(
      api.asc_findText(props) ||
        api.asc_findText(props, false, false) ||
        api.asc_findText(arg.marker, false, false),
    );
  } catch {
    return false;
  }
  if (!found) {
    return false;
  }
  if (arg.format.bold) {
    api.asc_setFontBold?.(true);
  }
  if (arg.format.italic) {
    const italicFns = ["asc_setFontItalic", "asc_SetTextItalic", "asc_setTextItalic"];
    for (const name of italicFns) {
      const fn = (api as Record<string, unknown>)[name];
      if (typeof fn === "function") {
        (fn as (value: boolean) => void).call(api, true);
      }
    }
  }
  if (arg.format.highlight) {
    if (api.asc_putHighlightColor) {
      api.asc_putHighlightColor("ffff00");
    } else {
      api.asc_putHighlight?.("ffff00");
    }
  }
  return true;
}

async function formatWordSelectionViaSearchUI(
  frame: FrameLocator,
  marker: string,
  format: WordFormatOptions,
): Promise<void> {
  await frame.locator("body").click({ position: { x: 10, y: 10 }, force: true }).catch(() => {});
  await frame.locator("body").press("Control+f");
  const searchInput = frame.locator("#search-bar-text").first();
  await searchInput.waitFor({ state: "visible", timeout: 8_000 });
  await searchInput.fill(marker);
  await searchInput.press("Enter");
  await settleFrame(frame, 400);
  await expect
    .poll(async () => searchBarHasMatches(frame), { timeout: 8_000, intervals: [200, 500] })
    .toBe(true);
  // Apply via toolbar while the search selection is still active (Escape clears it).
  if (format.bold) {
    const boldBtn = frame
      .locator(
        '#slot-btn-font-bold, #id-toolbar-btn-bold, [id*="font-bold"], button[aria-label*="Bold" i]',
      )
      .first();
    if ((await boldBtn.count()) > 0 && (await boldBtn.isVisible().catch(() => false))) {
      await boldBtn.click({ force: true });
    } else {
      await frame.locator("body").press("Control+b");
    }
  }
  if (format.italic) {
    const italicBtn = frame
      .locator(
        '#slot-btn-font-italic, #id-toolbar-btn-italic, [id*="font-italic"], button[aria-label*="Italic" i]',
      )
      .first();
    if ((await italicBtn.count()) > 0 && (await italicBtn.isVisible().catch(() => false))) {
      await italicBtn.click({ force: true });
    } else {
      await frame.locator("body").press("Control+i");
    }
  }
  if (format.highlight) {
    const highlightBtn = frame
      .locator('#slot-btn-highlight-color, #slot-btn-font-highlight, [id*="highlight"]')
      .first();
    if ((await highlightBtn.count()) > 0) {
      await highlightBtn.click({ force: true });
      const yellow = frame.locator('[data-color="ffff00"], [data-value="ffff00"], .color-yellow').first();
      if ((await yellow.count()) > 0) {
        await yellow.click({ force: true });
      }
    }
  }
  await closeSearchBar(frame);
}

/** Find marker text in a word document and apply formatting. */
export async function formatWordSelection(
  page: Page,
  marker: string,
  format: WordFormatOptions,
): Promise<void> {
  await waitForEditorEditable(page, "word");
  const frame = getEditorFrame(page, "word");
  const file = new URL(page.url()).searchParams.get("file") ?? "";
  const preferSearchUi = file.endsWith(".rtf");

  let sdkApplied = false;
  if (!preferSearchUi) {
    sdkApplied = await frame
      .locator("body")
      .evaluate(applyWordFormatInBrowser, { marker, format });
  }
  if (!sdkApplied || !(await documentContainsText(frame, marker, "word"))) {
    await formatWordSelectionViaSearchUI(frame, marker, format);
  }

  await expect
    .poll(async () => documentContainsText(frame, marker, "word"), {
      timeout: DOCUMENT_READY_TIMEOUT,
      intervals: [300, 500, 1000],
    })
    .toBe(true);
  await settleFrame(frame, 400);
}

export type RtfFormattingAssert = {
  marker: string;
  bold?: boolean;
  italic?: boolean;
  highlight?: boolean;
};

export async function assertDemoRtfFormatting(
  request: APIRequestContext,
  filePath: string,
  opts: RtfFormattingAssert,
): Promise<void> {
  const buf = await fetchDemoFileBody(request, filePath);
  const rtf = decodeOfficeText(buf);
  expect(rtfPlainText(rtf)).toContain(opts.marker);
  const window = rtfWindowAroundMarker(rtf, opts.marker);
  expect(window.length).toBeGreaterThan(0);
  if (opts.bold) {
    expect(window).toMatch(/\\b(?!ullet)/);
  }
  if (opts.italic) {
    expect(rtf).toMatch(/\\i(?!nfo|lvl|tap)[^a-zA-Z]/);
  }
  if (opts.highlight) {
    expect(rtf.match(/\\highlight\d*|\\cb\d+|\\chcbpat\d+/)).toBeTruthy();
  }
}

/** Poll viewer status for stableMs after save; fail on Error: prefix. */
export async function assertEditorStable(page: Page, stableMs = 15_000): Promise<void> {
  await expect
    .poll(
      async () => {
        const saveError = await page.locator("body").getAttribute("data-save-error");
        if (saveError) {
          return false;
        }
        const status = (await page.locator("#status").textContent()) ?? "";
        if (status.startsWith("Error:")) {
          return false;
        }
        const className = (await page.locator("#status").getAttribute("class")) ?? "";
        return !className.includes("status-error");
      },
      { timeout: stableMs, intervals: [500] },
    )
    .toBe(true);
}

/** Minimal dirty edit before save (word/slide append, cell A1). */
export async function applyMinimalSaveEdit(
  page: Page,
  editor: SampleFile["editor"],
  marker = "PW_STABLE",
): Promise<void> {
  if (editor === "cell") {
    await editCellForSave(page, editor, "A1", marker);
    return;
  }
  if (editor === "word" || editor === "slide") {
    await editWordForSave(page, editor, marker);
    return;
  }
  throw new Error(`unsupported editor for save edit: ${editor}`);
}

export async function fetchDemoFileBody(
  request: APIRequestContext,
  filePath: string,
): Promise<Buffer> {
  const res = await request.get(`/api/office/demo/file/${encodeURIComponent(filePath)}`, {
    headers: { "Cache-Control": "no-cache" },
  });
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

export async function editorWitness(
  page: Page,
  editor?: SampleFile["editor"],
): Promise<string> {
  const body = page.locator("body");
  const status = (await page.locator("#status").textContent()) ?? "";
  const witness: Record<string, unknown> = {
    warmDone: await body.getAttribute("data-warm-done"),
    documentReady: await body.getAttribute("data-document-ready"),
    contentReady: await body.getAttribute("data-content-ready"),
    dirty: await body.getAttribute("data-dirty"),
    saveDone: await body.getAttribute("data-save-done"),
    saveResult: await body.getAttribute("data-save-result"),
    saveError: await body.getAttribute("data-save-error"),
    saving: await body.getAttribute("data-saving"),
    status,
    statusClass: (await page.locator("#status").getAttribute("class")) ?? "",
  };
  if (editor) {
    try {
      const frame = getEditorFrame(page, editor);
      witness.iframeSrc = await page
        .locator(`iframe[src*="/${EDITOR_APP[editor]}/"]`)
        .first()
        .getAttribute("src");
      witness.shellReady = await isEditorShellReady(frame, editor);
      witness.loadMaskBlocking = await isLoadMaskBlocking(frame);
      witness.interactive = await isEditorInteractive(page, frame, editor);
      witness.editable = await isEditorEditable(page, frame, editor);
    } catch (err) {
      witness.frameProbeError = String(err);
    }
  }
  return JSON.stringify(witness, null, 2);
}

export async function waitForSaveDone(
  page: Page,
  opts?: SaveDoneOptions | number,
): Promise<void> {
  const options: SaveDoneOptions =
    typeof opts === "number" ? { timeoutMs: opts } : (opts ?? {});
  const timeoutMs = options.timeoutMs ?? SAVE_DONE_TIMEOUT;
  const requireMarker = Boolean(options.marker && options.filePath && options.request);

  try {
    await expect
      .poll(
        async () => {
          const saveError = await page.locator("body").getAttribute("data-save-error");
          const saveResult = await page.locator("body").getAttribute("data-save-result");
          if (saveError) {
            throw new Error(`save error: ${saveError}`);
          }
          if (saveResult === "error") {
            throw new Error("save result: error");
          }
          if (requireMarker) {
            const res = await options.request!.get(
              `/api/office/demo/file/${encodeURIComponent(options.filePath!)}`,
              { headers: { "Cache-Control": "no-cache" } },
            );
            if (res.ok() && officeFileContains(Buffer.from(await res.body()), options.marker!)) {
              return true;
            }
            return false;
          }
          const status = (await page.locator("#status").textContent()) ?? "";
          if (status.startsWith("Error:")) {
            throw new Error(`viewer status: ${status}`);
          }
          const saveDone = await page.locator("body").getAttribute("data-save-done");
          return Boolean(saveDone) || status.includes("Saved");
        },
        { timeout: timeoutMs },
      )
      .toBe(true);
  } catch (err) {
    throw new Error(`${String(err)}\nwitness:\n${await editorWitness(page)}`);
  }
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
