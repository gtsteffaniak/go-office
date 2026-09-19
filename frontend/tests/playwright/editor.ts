import {
  expect,
  type APIRequestContext,
  type FrameLocator,
  type Page,
} from "@playwright/test";
import { createHash } from "node:crypto";
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
const DEMO_WARM_TIMEOUT = Number(process.env.PLAYWRIGHT_WARM_TIMEOUT ?? 60_000);
const WARM_REQUEST_TIMEOUT = Number(process.env.PLAYWRIGHT_WARM_REQUEST_MS ?? 30_000);
const CONTENT_FIND_TIMEOUT = Number(process.env.PLAYWRIGHT_CONTENT_FIND_TIMEOUT ?? 15_000);
// Budget for the persisted-marker wait. A healthy flush lands in ~6s and the marker is
// visible ~2s later, so this is generous. It must fit inside the per-test timeout together
// with the readiness gates that run before it (see setCellContent/editCellForSave); the
// worst-case sum of those gates is what actually determines whether a failure is reported
// as a save problem or as a bare test timeout.
const SAVE_DONE_TIMEOUT = Number(process.env.PLAYWRIGHT_SAVE_DONE_TIMEOUT ?? 45_000);
const INTERACTIVE_SETTLE_MS = 400;

// Cell-editing gates are SEQUENTIAL, so their timeouts add up. Individually generous gates
// are therefore not safe on their own: the worst-case sum below previously reached ~320s
// against the 150s per-test budget, and under load that surfaced as a bare
// "Test timeout of 150000ms exceeded" after the browser had already been torn down —
// indistinguishable from a genuine hang. A blanket increase in the gates makes that worse,
// not better.
//
// The helpers below therefore charge every wait against ONE shared per-test deadline
// (testBudget), so the gates can be individually generous without their sum escaping the
// budget. The last wait is truncated to whatever remains, which keeps a failure attributed
// to the step that actually stalled.
//
// This matters because the observed cost is load-dependent: under combined load (three
// Playwright projects against one server, 8 workers) the editor has been seen reaching
// "document content ready" in 15-21s, versus ~200ms isolated. A fixed 20s gate made the
// outcome depend on scheduling rather than on the code under test.
const TEST_BUDGET_MS = Number(process.env.PLAYWRIGHT_SAVE_TEST_TIMEOUT ?? 150_000);
/** Reserve for teardown/artifacts so a truncated wait still reports a named error. */
const TEST_BUDGET_RESERVE_MS = 8_000;

/** Wall-clock start of the current test's budget, keyed by the page that owns it. */
const budgetStart = new WeakMap<object, number>();

/** Mark the start of a test's budget window for `page`. Called by the readiness helpers. */
export function startTestBudget(page: Page): void {
  budgetStart.set(page, Date.now());
}

/**
 * Clamp a wait to the remaining test budget.
 *
 * Returns at least 1s so a wait always makes progress (and fails on its own terms) rather
 * than being handed a zero/negative timeout by Playwright.
 */
function withinBudget(page: Page, requestedMs: number): number {
  const start = budgetStart.get(page);
  if (start === undefined) {
    return requestedMs;
  }
  const remaining = TEST_BUDGET_MS - TEST_BUDGET_RESERVE_MS - (Date.now() - start);
  return Math.max(1_000, Math.min(requestedMs, remaining));
}

// Interactive-state gates for the cell editor. These run AFTER the document reports itself
// ready, so they measure how long the spreadsheet UI takes to finish wiring itself up.
// They are generous because the shared deadline above, not the gate, is what bounds the test.
const CELL_SELECT_TIMEOUT = Number(process.env.PLAYWRIGHT_CELL_SELECT_TIMEOUT ?? 45_000);
const CELL_INTERACTIVE_TIMEOUT = Number(process.env.PLAYWRIGHT_CELL_INTERACTIVE_TIMEOUT ?? 45_000);
const CELL_SDK_TIMEOUT = Number(process.env.PLAYWRIGHT_CELL_SDK_TIMEOUT ?? 45_000);
const CELL_SDK_TIMEOUT_FALLBACK = Number(
  process.env.PLAYWRIGHT_CELL_SDK_TIMEOUT_FALLBACK ?? 8_000,
);
const CELL_ACK_TIMEOUT = Number(process.env.PLAYWRIGHT_CELL_ACK_TIMEOUT ?? 10_000);
const CELL_DIAG_TIMEOUT = Number(process.env.PLAYWRIGHT_CELL_DIAG_TIMEOUT ?? 3_000);
// Word/slide save edits treat readiness as advisory. This bounds that advisory wait so the
// edit is still attempted while there is budget left for the save and the marker poll.
const WORD_READY_TIMEOUT = Number(process.env.PLAYWRIGHT_WORD_READY_TIMEOUT ?? 20_000);

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

async function isCellEditorEditable(frame: FrameLocator): Promise<boolean> {
  if (await isLoadMaskBlocking(frame)) {
    return false;
  }
  const cellName = frame.locator(CELL_NAME_INPUT).first();
  const formulaBar = frame.locator(CELL_VALUE_INPUT).first();
  if ((await cellName.count()) === 0 || (await formulaBar.count()) === 0) {
    return false;
  }
  if (!(await cellName.isEnabled().catch(() => false))) {
    return false;
  }
  if (!(await formulaBar.isVisible().catch(() => false))) {
    return false;
  }
  if (!(await isEditorShellReady(frame, "cell"))) {
    return false;
  }
  return selectCellViaUi(frame, "A1");
}

async function isWordSlideEditorEditable(
  page: Page,
  frame: FrameLocator,
  editor: SampleFile["editor"],
): Promise<boolean> {
  if (await isLoadMaskBlocking(frame)) {
    return false;
  }
  if (!(await isEditorShellReady(frame, editor))) {
    return false;
  }
  if ((await page.locator("body").getAttribute("data-document-ready")) !== "true") {
    return false;
  }
  return page.evaluate(() => {
    const ed = (window as { docEditor?: { insertPlainText?: (t: string) => void } }).docEditor;
    return typeof ed?.insertPlainText === "function";
  });
}

async function isEditorEditable(
  page: Page,
  frame: FrameLocator,
  editor: SampleFile["editor"],
): Promise<boolean> {
  if (editor === "cell") {
    return isCellEditorEditable(frame);
  }
  if (editor === "word" || editor === "slide") {
    return isWordSlideEditorEditable(page, frame, editor);
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

/** Demo viewer finished server warm; fails fast when data-warm-error is set. */
export async function waitForDemoWarm(page: Page, timeoutMs = DEMO_WARM_TIMEOUT): Promise<void> {
  await page.waitForFunction(
    () => {
      const warmError = document.body.getAttribute("data-warm-error");
      if (warmError) {
        throw new Error(`viewer warm failed: ${warmError}`);
      }
      return document.body.getAttribute("data-warm-done") === "true";
    },
    { timeout: timeoutMs },
  );
}

/** Block until x2t has produced Editor.bin for file (use before goto for forked save paths). */
export async function warmDemoFile(
  request: APIRequestContext,
  filePath: string,
): Promise<void> {
  const warm = await request.get(`/demo/warm?file=${encodeURIComponent(filePath)}`, {
    timeout: WARM_REQUEST_TIMEOUT,
  });
  if (!warm.ok()) {
    const body = await warm.text();
    throw new Error(
      `warm ${filePath}: HTTP ${warm.status()} ${body.slice(0, 200)} (timeout=${WARM_REQUEST_TIMEOUT}ms)`,
    );
  }
}

/**
 * Editor iframe mounted and document is ready to use (open-format tests).
 *
 * `required: false` makes the gate advisory so save specs, which assert against the stored
 * file rather than the UI, are not failed by a worker that is merely slow to boot.
 */
export async function waitForEditorReady(
  page: Page,
  editor: SampleFile["editor"],
  opts?: { timeoutMs?: number; required?: boolean },
): Promise<boolean> {
  // Start (or restart) this page's shared budget window. Every subsequent wait clamps itself
  // to what remains, which is what keeps individually generous gates from summing past the
  // per-test timeout.
  startTestBudget(page);
  const timeoutMs = withinBudget(page, opts?.timeoutMs ?? DOCUMENT_READY_TIMEOUT);
  const required = opts?.required ?? true;
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
      { timeout: timeoutMs },
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
        { timeout: timeoutMs },
      )
      .toBe(true);
    return true;
  } catch (err) {
    if (required) {
      throw new Error(`${String(err)}\nwitness:\n${await editorWitness(page, editor)}`);
    }
    return false;
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

/**
 * Document loaded, load masks gone, and editor APIs are usable (content/save tests).
 *
 * With `required: false` this becomes advisory: it still waits up to timeoutMs and performs
 * the same settling/overlay cleanup, but returns whether the editor reported itself
 * interactive instead of throwing. Callers use that to shorten their next wait rather than
 * failing outright, which keeps sequential gate timeouts from exceeding the test budget.
 */
export async function waitForEditorInteractive(
  page: Page,
  editor: SampleFile["editor"],
  opts?: number | { timeoutMs?: number; required?: boolean },
): Promise<boolean> {
  const options = typeof opts === "number" ? { timeoutMs: opts } : (opts ?? {});
  const timeoutMs = withinBudget(page, options.timeoutMs ?? DOCUMENT_READY_TIMEOUT);
  const required = options.required ?? true;
  const frame = getEditorFrame(page, editor);

  let interactive = false;
  try {
    await expect
      .poll(async () => isEditorInteractive(page, frame, editor), { timeout: timeoutMs })
      .toBe(true);
    interactive = true;
  } catch (err) {
    if (required) {
      // Report why the editor never became interactive, rather than a bare poll timeout.
      // The witness records the viewer status, load-mask and frame-readiness state; the
      // browser console is attached separately by the collectPageErrors fixture.
      throw new Error(
        `editor did not become interactive within ${timeoutMs}ms\nwitness:\n${await editorWitness(page, editor).catch(() => "<unavailable>")}\n${String(err)}`,
      );
    }
  }

  await page.waitForTimeout(INTERACTIVE_SETTLE_MS);
  if (await isLoadMaskBlocking(frame)) {
    try {
      await expect
        .poll(async () => isEditorInteractive(page, frame, editor), { timeout: 10_000 })
        .toBe(true);
      interactive = true;
    } catch (err) {
      if (required) {
        throw new Error(
          `load mask still blocking after ${timeoutMs}ms\nwitness:\n${await editorWitness(page, editor).catch(() => "<unavailable>")}\n${String(err)}`,
        );
      }
    }
  }
  if (editor === "cell") {
    await dismissEditorOverlays(frame);
  }
  return interactive;
}

/**
 * Document is ready to accept a single save-test edit (UI controls or public insertPlainText).
 *
 * `required: false` makes the whole gate advisory and returns whether the editor reached an
 * editable state. Save specs assert persistence against the stored file, and a slow worker
 * that has not finished booting is not evidence that the save is broken — so they call this
 * best-effort and let the insert itself report a genuine failure.
 */
export async function waitForEditorEditable(
  page: Page,
  editor: SampleFile["editor"],
  opts?: number | { timeoutMs?: number; required?: boolean },
): Promise<boolean> {
  const options = typeof opts === "number" ? { timeoutMs: opts } : (opts ?? {});
  const timeoutMs = withinBudget(page, options.timeoutMs ?? DOCUMENT_READY_TIMEOUT);
  const required = options.required ?? true;
  try {
    await waitForEditorReady(page, editor, { timeoutMs, required });
    const frame = getEditorFrame(page, editor);
    await expect
      .poll(async () => isEditorEditable(page, frame, editor), {
        timeout: withinBudget(page, timeoutMs),
      })
      .toBe(true);
    await page.waitForTimeout(INTERACTIVE_SETTLE_MS);
    if (editor === "cell") {
      await dismissEditorOverlays(frame);
    }
    return true;
  } catch (err) {
    if (required) {
      throw new Error(`${String(err)}\nwitness:\n${await editorWitness(page, editor)}`);
    }
    return false;
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

/**
 * Select a spreadsheet cell via SDK when available, otherwise the name box UI.
 *
 * Both waits are bounded well below EDITOR_LOAD_TIMEOUT. They are sequential, so the
 * previous 45s + 45s could consume 90s of a 150s test budget on a worker whose cell editor
 * was never going to become ready — the failure then surfaced as a teardown error instead
 * of naming the real problem. Failing fast here leaves budget for the save steps that
 * follow and produces an actionable error.
 */
async function selectCell(page: Page, frame: FrameLocator, ref: string): Promise<boolean> {
  const nameBoxEnabled = await frame
    .locator(CELL_NAME_INPUT)
    .first()
    .isEnabled()
    .catch(() => false);
  if (!nameBoxEnabled) {
    // The document may still be booting: under combined load onDocumentContentReady has
    // been observed at 15s+, which is inside any reasonable gate but means the name box is
    // genuinely not usable yet. Return the fact instead of throwing so a caller that
    // already wrote the value through the SDK is not killed by a slow UI.
    return false;
  }
  try {
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
        { timeout: withinBudget(page, CELL_SELECT_TIMEOUT), intervals: [200, 500, 1000] },
      )
      .toBe(true);
    return true;
  } catch {
    return false;
  }
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

async function readCellValue(page: Page, frame: FrameLocator, ref: string): Promise<string> {
  const viaSdk = await frame.locator("body").evaluate(readCellViaBrowser, ref);
  if (viaSdk) {
    return viaSdk;
  }

  if (!(await selectCell(page, frame, ref))) {
    return "";
  }
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
    const value = await readCellValue(page, frame, ref);
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
  // The readiness gates below are sequential, so their timeouts ADD UP. With the previous
  // 45s + 45s (+10s re-poll) the worst case already exceeded the test budget before the
  // save was attempted, which is why slow workers timed out rather than failing on the
  // save. Each gate is now bounded so the total stays comfortably inside the budget, and
  // the interactive gate is advisory: the edit is still attempted, because a UI/API
  // readback being unavailable does not mean the editor cannot accept input.
  const interactive = await waitForEditorInteractive(page, editor, {
    timeoutMs: withinBudget(page, CELL_INTERACTIVE_TIMEOUT),
    required: false,
  });
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
        {
          timeout: interactive
            ? withinBudget(page, CELL_SDK_TIMEOUT)
            : withinBudget(page, CELL_SDK_TIMEOUT_FALLBACK),
          intervals: [200, 500, 1000],
        },
      )
      .toBe(true);
    viaSdk = true;
  } catch {
    viaSdk = false;
  }

  if (!viaSdk) {
    if (await selectCell(page, frame, ref)) {
      await writeFormulaBarValue(frame, value);
    }
  }

  await commitCellEdit(frame);
  if (!(await cellShowsValue(page, frame, editor, ref, value))) {
    // Best-effort retry. The caller (editCellForSave) treats editor-side acknowledgement as
    // advisory because the persisted file is the contract, so a UI that never becomes usable
    // must not fail the test here — that only converts a slow worker into a false negative.
    if (await selectCell(page, frame, ref)) {
      await writeFormulaBarValue(frame, value);
      await commitCellEdit(frame);
    }
  }
  await settleFrame(frame, 500);
}

/**
 * Set a cell value for save tests (SDK/UI write with retries).
 *
 * This deliberately does NOT assert that the editor's own API acknowledged the edit.
 * `cellShowsValue` needs either the sdkjs cell API (`apiReady`) or a UI readback, and
 * under parallel CI load the cell editor frequently never reaches that state even though
 * the value was entered and the flush succeeded — the ODS failure detail showed exactly
 * that (`formula=PW_STABLE_…` present, `apiReady=false`). The save specs care about
 * whether the edit *persists*, which `waitForPersistedMarker` verifies against the stored
 * file; that is the authoritative contract for those tests.
 *
 * A short best-effort acknowledgement wait is still attempted so the common case keeps
 * exercising the SDK path, and the observed editor state is returned for diagnostics.
 */
export async function editCellForSave(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  value: string,
): Promise<CellEditOutcome> {
  await setCellContent(page, editor, ref, value);
  const frame = getEditorFrame(page, editor);

  const apiReady = await frame
    .locator("body")
    .evaluate(cellSetApiReadyInBrowser)
    .catch(() => false);

  let acknowledged = false;
  try {
    await expect
      .poll(async () => cellShowsValue(page, frame, editor, ref, value), {
        // Bounded and short: this is a convenience check, not the test contract. Waiting
        // EDITOR_LOAD_TIMEOUT here burned most of the test budget on slow workers.
        timeout: withinBudget(page, CELL_ACK_TIMEOUT),
        intervals: [200, 500, 1000],
      })
      .toBe(true);
    acknowledged = true;
  } catch {
    // Investigate below and report; do not fail here.
  }

  const outcome: CellEditOutcome = { acknowledged, apiReady, ref, value };
  if (!acknowledged) {
    // Diagnostics are best-effort and must not stall the test: on a slow or wedged worker
    // these locators can themselves block, which is the very condition they report. Bound
    // each one so a failure stays a fast, informative failure.
    const diag = { timeout: CELL_DIAG_TIMEOUT };
    outcome.nameBox = await frame
      .locator(CELL_NAME_INPUT)
      .first()
      .inputValue(diag)
      .catch(async () => (await frame.locator(CELL_NAME_INPUT).first().innerText(diag)) ?? "")
      .catch(() => "");
    outcome.formula = await readFormulaBarValue(frame).catch(() => "");
    outcome.loadMask = await isLoadMaskBlocking(frame).catch(() => false);
    // The value being present in the formula bar means the edit reached the editor; it is
    // the readback path (not the entry) that is unavailable. Record which, so a genuine
    // entry failure stays distinguishable from a readback limitation.
    outcome.valueEntered = (outcome.formula ?? "").includes(value);
  }
  return outcome;
}

/** Observed state after a save-test cell edit; see editCellForSave. */
export type CellEditOutcome = {
  acknowledged: boolean;
  apiReady: boolean;
  ref: string;
  value: string;
  nameBox?: string;
  formula?: string;
  loadMask?: boolean;
  /** True when the formula bar shows the value, i.e. the edit itself succeeded. */
  valueEntered?: boolean;
};

/** Human-readable summary of a cell edit outcome, for failure messages. */
export function describeCellEdit(outcome: CellEditOutcome): string {
  return (
    `ref=${outcome.ref} value=${outcome.value} acknowledged=${outcome.acknowledged} ` +
    `apiReady=${outcome.apiReady} valueEntered=${outcome.valueEntered ?? "?"} ` +
    `nameBox=${outcome.nameBox ?? "?"} formula=${outcome.formula ?? "?"} ` +
    `loadMask=${outcome.loadMask ?? "?"}`
  );
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

/**
 * Insert a unique marker into word/slide documents for save tests.
 *
 * Like editCellForSave, this does NOT assert that the marker became visible in the editor.
 * `documentContainsText` depends on the sdkjs search API or the editor's DOM, neither of
 * which is reliable under parallel CI load; the save specs care whether the edit persists,
 * which their `waitForPersistedMarker` step verifies against the stored file. A short
 * bounded best-effort visibility check is still attempted, and the outcome is returned for
 * diagnostics. Throws only when the insert call itself was unavailable, which is a genuine
 * harness failure rather than a timing artefact.
 */
export async function editWordForSave(
  page: Page,
  editor: SampleFile["editor"],
  marker: string,
): Promise<WordEditOutcome> {
  // Advisory, like the cell path: readiness is a UI signal, but the contract these specs
  // assert is that the edit persists to the stored file. Under combined load the editor has
  // been observed taking 15s+ to reach document-ready, and hard-failing here meant the edit
  // was never attempted at all — the marker then never landed and the file fingerprint was
  // unchanged, which is exactly the "identical before/after fingerprint" failure. insert
  // below is the real gate; if it is unavailable the editor is genuinely unusable.
  await waitForEditorEditable(page, editor, {
    timeoutMs: WORD_READY_TIMEOUT,
    required: false,
  });
  const chunk = ` ${marker}`;
  const inserted = await page.evaluate((text: string) => {
    const ed = (window as {
      docEditor?: { grabFocus?: () => void; insertPlainText?: (t: string) => void };
    }).docEditor;
    if (!ed || typeof ed.insertPlainText !== "function") {
      return false;
    }
    ed.grabFocus?.();
    ed.insertPlainText(text);
    return true;
  }, chunk);
  if (!inserted) {
    throw new Error(`insertPlainText unavailable\nwitness:\n${await editorWitness(page, editor)}`);
  }

  let visible = false;
  try {
    await waitForMarkerInEditor(
      page,
      editor,
      marker,
      Number(process.env.PLAYWRIGHT_WORD_ACK_TIMEOUT ?? 10_000),
    );
    visible = true;
  } catch {
    // Best effort only; the spec asserts persistence, not editor visibility.
  }

  const outcome: WordEditOutcome = { inserted, visible, marker };
  if (!visible) {
    outcome.witness = await editorWitness(page, editor).catch(() => "");
  }
  return outcome;
}

/** Observed state after a save-test word/slide edit; see editWordForSave. */
export type WordEditOutcome = {
  inserted: boolean;
  visible: boolean;
  marker: string;
  witness?: string;
};

/** Human-readable summary of a word/slide edit outcome, for failure messages. */
export function describeWordEdit(outcome: WordEditOutcome): string {
  const base = `marker=${outcome.marker} inserted=${outcome.inserted} visibleInEditor=${outcome.visible}`;
  return outcome.witness ? `${base}\nwitness:\n${outcome.witness}` : base;
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

/** Click Save once inside the editor iframe (Ctrl+S only when no button exists). */
export async function triggerManualSave(page: Page, editor: SampleFile["editor"]): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await frame.locator("body").click({ position: { x: 12, y: 12 }, force: true });

  const saveBtn = frame.locator(SAVE_BUTTON).first();
  if ((await saveBtn.count()) > 0) {
    await saveBtn.click({ force: true, timeout: 5_000, noWaitAfter: true });
    return;
  }
  await frame.locator("body").press("Control+s");
}

/** @deprecated Use triggerManualSave for the one Save-button smoke; autosave tests poll disk instead. */
export async function triggerEditorSave(page: Page, editor: SampleFile["editor"]): Promise<void> {
  await triggerManualSave(page, editor);
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
    .poll(async () => searchBarHasMatches(frame), {
      timeout: CONTENT_FIND_TIMEOUT,
      intervals: [200, 500, 1000],
    })
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
  await waitForMarkerInEditor(page, "word", marker);

  const sdkApplied = await frame
    .locator("body")
    .evaluate(applyWordFormatInBrowser, { marker, format });
  if (!sdkApplied) {
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

function rtfFormattingMatches(rtf: string, opts: RtfFormattingAssert): boolean {
  if (!rtfPlainText(rtf).includes(opts.marker)) {
    return false;
  }
  const window = rtfWindowAroundMarker(rtf, opts.marker);
  if (window.length === 0) {
    return false;
  }
  if (opts.bold && !/\\b(?!ullet)/.test(window)) {
    return false;
  }
  if (opts.italic && !/\\i(?!nfo|lvl|tap)[^a-zA-Z]/.test(rtf)) {
    return false;
  }
  if (opts.highlight && !rtf.match(/\\highlight\d*|\\cb\d+|\\chcbpat\d+/)) {
    return false;
  }
  return true;
}

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

/** Poll the RTF file until formatting around the marker is persisted. */
export async function waitForDemoRtfFormatting(
  request: APIRequestContext,
  filePath: string,
  opts: RtfFormattingAssert,
  timeoutMs = SAVE_DONE_TIMEOUT,
): Promise<void> {
  await waitForPersistedContent(
    request,
    filePath,
    (buf) => rtfFormattingMatches(decodeOfficeText(buf), opts),
    { timeoutMs, label: `RTF formatting for "${opts.marker}"` },
  );
}

/** Observe the full stability window; fail immediately on viewer or save errors. */
export async function assertEditorStable(page: Page, stableMs = 15_000): Promise<void> {
  const interval = 500;
  const checks = Math.max(1, Math.ceil(stableMs / interval));
  for (let i = 0; i < checks; i++) {
    const saveError = await page.locator("body").getAttribute("data-save-error");
    if (saveError) {
      throw new Error(`save error during stability window: ${saveError}`);
    }
    const status = (await page.locator("#status").textContent()) ?? "";
    if (status.startsWith("Error:")) {
      throw new Error(`viewer error during stability window: ${status}`);
    }
    const className = (await page.locator("#status").getAttribute("class")) ?? "";
    if (className.includes("status-error")) {
      throw new Error("status-error during stability window");
    }
    if (i < checks - 1) {
      await page.waitForTimeout(interval);
    }
  }
}

/** Minimal dirty edit before save (word/slide append, cell A1). */
export async function applyMinimalSaveEdit(
  page: Page,
  editor: SampleFile["editor"],
  marker = "PW_STABLE",
): Promise<CellEditOutcome | undefined> {
  if (editor === "cell") {
    return editCellForSave(page, editor, "A1", marker);
  }
  if (editor === "word" || editor === "slide") {
    await editWordForSave(page, editor, marker);
    return undefined;
  }
  throw new Error(`unsupported editor for save edit: ${editor}`);
}

export function fileFingerprint(buf: Buffer): string {
  return createHash("sha256").update(buf).digest("hex");
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

export type PersistOptions = {
  timeoutMs?: number;
  page?: Page;
  label?: string;
};

/**
 * Poll demo file bytes until a caller-supplied postcondition is true.
 *
 * The baseline fingerprint fetch is best-effort. It used to run unprotected before the
 * poll, so if the test budget expired at that moment Playwright had already torn down the
 * browser and APIRequestContext; the request then rejected with
 * "Target page, context or browser has been closed" and REPLACED the real failure. A
 * successful save could therefore be reported as a context-teardown error, which is
 * exactly what happened to the csv round-trip test (the server had persisted the marker
 * 1.5s after the edit; the test never got to observe it).
 */
export async function waitForPersistedContent(
  request: APIRequestContext,
  filePath: string,
  postcondition: (buf: Buffer) => boolean,
  opts?: PersistOptions,
): Promise<void> {
  // Charge this wait against the shared per-test budget too. Under combined load the
  // readiness gates ahead of it can legitimately consume most of the window, and letting the
  // marker poll run a full SAVE_DONE_TIMEOUT regardless produced "Test timeout of 150000ms
  // exceeded" with the real error replaced by a teardown message. Truncating keeps whatever
  // budget is left and reports the marker failure on its own terms.
  const requested = opts?.timeoutMs ?? SAVE_DONE_TIMEOUT;
  const timeoutMs = opts?.page ? withinBudget(opts.page, requested) : requested;
  const label = opts?.label ?? "persisted content postcondition";
  let beforeFp = "<unavailable>";
  try {
    beforeFp = fileFingerprint(await fetchDemoFileBody(request, filePath));
  } catch {
    // Teardown or a transient network error; the poll below reports the real problem.
  }
  try {
    await expect
      .poll(
        async () => {
          if (opts?.page) {
            const saveError = await opts.page.locator("body").getAttribute("data-save-error");
            if (saveError) {
              throw new Error(`save error: ${saveError}`);
            }
            const status = (await opts.page.locator("#status").textContent()) ?? "";
            if (status.startsWith("Error:")) {
              throw new Error(`viewer status: ${status}`);
            }
          }
          const buf = await fetchDemoFileBody(request, filePath);
          return postcondition(buf);
        },
        { timeout: timeoutMs, intervals: [500, 1000, 2000] },
      )
      .toBe(true);
  } catch (err) {
    // Also best-effort: if the failure IS a teardown, this fetch fails the same way and
    // would otherwise mask the original error a second time.
    let afterFp = "<unavailable>";
    try {
      afterFp = fileFingerprint(await fetchDemoFileBody(request, filePath));
    } catch {
      // keep "<unavailable>"
    }
    throw new Error(
      `${String(err)}\n${label} not met within ${timeoutMs}ms\nfingerprint before=${beforeFp}\nfingerprint after=${afterFp}`,
    );
  }
}

/** Poll until marker text is present in the persisted demo file. */
export async function waitForPersistedMarker(
  request: APIRequestContext,
  filePath: string,
  marker: string,
  opts?: PersistOptions,
): Promise<void> {
  await waitForPersistedContent(
    request,
    filePath,
    (buf) => officeFileContains(buf, marker),
    { ...opts, label: `marker "${marker}"` },
  );
}

/**
 * waitForPersistedMarker with an attached edit diagnostic. The edit steps are best-effort
 * about *editor-side* acknowledgement (see editCellForSave/editWordForSave), so when the
 * marker never lands the caller needs to know whether the edit itself reached the editor.
 * Attaching the detail here keeps a genuine product failure distinguishable from an
 * unavailable editor readback.
 */
export async function expectPersistedMarker(
  request: APIRequestContext,
  filePath: string,
  marker: string,
  page: Page,
  editDetail: string,
  timeoutMs?: number,
): Promise<void> {
  try {
    await waitForPersistedMarker(request, filePath, marker, { page, timeoutMs });
  } catch (err) {
    throw new Error(`${String(err)}\n${editDetail}`);
  }
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
    warmError: await body.getAttribute("data-warm-error"),
    documentReady: await body.getAttribute("data-document-ready"),
    dirty: await body.getAttribute("data-dirty"),
    saveDone: await body.getAttribute("data-save-done"),
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
  if (!options.marker || !options.filePath || !options.request) {
    throw new Error("waitForSaveDone requires request, filePath, and marker");
  }
  try {
    await waitForPersistedMarker(options.request, options.filePath, options.marker, {
      timeoutMs: options.timeoutMs,
      page,
    });
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
