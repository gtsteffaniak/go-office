import { expect, type Page } from "@playwright/test";
import type { SampleFile } from "./samples";

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

/**
 * DocsAPI mounts the Euro-Office editor in an app iframe (document/cell/slide/pdf).
 * The shell divs (#id_main, #editor-container, …) live inside that frame.
 */
export async function waitForEditorShell(
  page: Page,
  editor: SampleFile["editor"],
): Promise<void> {
  const app = EDITOR_APP[editor];
  const appFrame = page.frameLocator(`iframe[src*="/${app}/"]`).first();

  await expect(appFrame.locator(EDITOR_SHELL).first()).toBeVisible({
    timeout: EDITOR_LOAD_TIMEOUT,
  });
}
