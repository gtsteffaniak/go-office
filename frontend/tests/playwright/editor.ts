import { expect, type FrameLocator, type Page } from "@playwright/test";
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
  process.env.PLAYWRIGHT_EDITOR_TIMEOUT ?? (bundledTest ? 60_000 : 30_000),
);
const DOCUMENT_READY_TIMEOUT = Number(
  process.env.PLAYWRIGHT_DOCUMENT_READY_TIMEOUT ?? (bundledTest ? 90_000 : 45_000),
);

export function getEditorFrame(page: Page, editor: SampleFile["editor"]): FrameLocator {
  const app = EDITOR_APP[editor];
  return page.frameLocator(`iframe[src*="/${app}/"]`).first();
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

/** Wait until the demo viewer reports document ready (see internal/demo/viewer.html). */
export async function waitForDocumentReady(page: Page): Promise<void> {
  await expect(page.locator("body")).toHaveAttribute("data-document-ready", "true", {
    timeout: DOCUMENT_READY_TIMEOUT,
  });
}

const FORMULA_BAR_SELECTORS = [
  "#ce-cell-name",
  "#id_formula",
  "#id_formula_input",
  ".formula-input input",
  ".formula-input textarea",
  "input.formula",
  "textarea.formula",
];

async function readFormulaBar(frame: FrameLocator): Promise<string> {
  for (const sel of FORMULA_BAR_SELECTORS) {
    const loc = frame.locator(sel).first();
    if ((await loc.count()) === 0) {
      continue;
    }
    try {
      const value = await loc.inputValue({ timeout: 2_000 });
      if (value !== undefined) {
        return value;
      }
    } catch {
      const text = await loc.textContent({ timeout: 2_000 });
      if (text) {
        return text;
      }
    }
  }
  return frame.evaluate(() => {
    const w = window as unknown as {
      Asc?: { editor?: { asc_getCellText?: () => string } };
    };
    return w.Asc?.editor?.asc_getCellText?.() ?? "";
  });
}

async function selectCell(frame: FrameLocator, ref: string): Promise<void> {
  for (const sel of ["#ce-cell-name", "#id_cell_name", ".cell-name input"]) {
    const loc = frame.locator(sel).first();
    if ((await loc.count()) === 0) {
      continue;
    }
    await loc.click();
    await loc.fill(ref);
    await loc.press("Enter");
    await frame.page().waitForTimeout(300);
    return;
  }

  await frame.evaluate((cellRef) => {
    const w = window as unknown as {
      Asc?: { editor?: { asc_selectRange?: (r: string) => void } };
    };
    w.Asc?.editor?.asc_selectRange?.(cellRef);
  }, ref);
  await frame.page().waitForTimeout(300);
}

export async function assertCellContent(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  expected: string,
): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await selectCell(frame, ref);
  const value = await readFormulaBar(frame);
  expect(value, `cell ${ref} formula bar`).toContain(expected);
}

export async function assertDocumentContains(
  page: Page,
  editor: SampleFile["editor"],
  text: string,
): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await frame.locator("#id_main, #editor_sdk, #editor-container").first().click();

  await page.keyboard.press("Control+KeyF");
  const findInput = frame.locator('input[type="search"], input[placeholder*="Find" i]').first();
  if ((await findInput.count()) > 0) {
    await findInput.fill(text);
    await expect(frame.getByText(text).first()).toBeVisible({ timeout: 15_000 });
    await page.keyboard.press("Escape");
    return;
  }

  const hasText = await frame.evaluate((needle) => document.body.innerText.includes(needle), text);
  expect(hasText, `document should contain ${text}`).toBe(true);
}

export async function setCellContent(
  page: Page,
  editor: SampleFile["editor"],
  ref: string,
  value: string,
): Promise<void> {
  const frame = getEditorFrame(page, editor);
  await selectCell(frame, ref);
  for (const sel of FORMULA_BAR_SELECTORS) {
    const loc = frame.locator(sel).first();
    if ((await loc.count()) === 0) {
      continue;
    }
    await loc.click();
    await loc.fill(value);
    await loc.press("Enter");
    return;
  }
  throw new Error("formula bar input not found for cell edit");
}

export async function waitForSaveDone(page: Page, timeoutMs = 30_000): Promise<void> {
  await expect
    .poll(async () => page.locator("body").getAttribute("data-save-done"), { timeout: timeoutMs })
    .not.toBeNull();
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
