import { test, expect } from "../test-setup";
import { getEditorFrame, warmDemoFile, waitForEditorInteractive, waitForEditorReady } from "../editor";

test.describe.configure({ mode: "serial" });

test("browser spellcheck loads dictionaries and wasm", async ({ page, request }) => {
  const dicRes = await request.get("/dictionaries/en_US/en_US.dic");
  expect(dicRes.status()).toBe(200);
  const deRes = await request.get("/dictionaries/de_DE/de_DE.dic");
  expect(deRes.status()).toBe(200);
  const wasmRes = await request.get("/sdkjs/common/spell/spell/spell.wasm");
  expect(wasmRes.status()).toBe(200);
  expect(wasmRes.headers()["content-type"] ?? "").toContain("wasm");

  await warmDemoFile(request, "sample-files/sample.docx");
  await page.goto("/demo/view?file=sample-files%2Fsample.docx");
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  const browserAssets = await page.evaluate(async () => {
    const aff = await fetch("/dictionaries/en_US/en_US.aff");
    const wasm = await fetch("/sdkjs/common/spell/spell/spell.wasm");
    return { aff: aff.ok, wasm: wasm.ok };
  });
  expect(browserAssets.aff).toBe(true);
  expect(browserAssets.wasm).toBe(true);

  const frame = getEditorFrame(page, "word");
  await expect
    .poll(
      async () =>
        frame
          .locator("body")
          .evaluate(() => Boolean((window as { Asc?: { editor?: { SpellCheckApi?: { worker?: unknown } } } }).Asc?.editor?.SpellCheckApi?.worker)),
      { timeout: 45_000 },
    )
    .toBe(true);

  await frame.locator("#id_main, #editor_sdk, #id_view").first().click({ timeout: 15_000 });
  await frame.locator("body").evaluate(() => {
    const api = (window as { Asc?: { editor?: { asc_enterText?: (t: string) => void } } }).Asc?.editor;
    api?.asc_enterText?.(" zzqwxteh ");
  });
  await page.waitForTimeout(2_000);

  await page.screenshot({
    path: "test-results/spellcheck-underline.png",
    fullPage: true,
  });

  const variants = await frame.locator("body").evaluate(async () => {
    const api = (window as { Asc?: { editor?: Record<string, unknown> } }).Asc?.editor;
    if (!api) {
      return [] as string[];
    }
    return await new Promise<string[]>((resolve) => {
      const timeout = window.setTimeout(() => resolve([]), 8_000);
      api.asc_onSpellCheckVariantsFound = (_props: unknown, list: string[]) => {
        if (list?.length) {
          window.clearTimeout(timeout);
          resolve(list);
        }
      };
    });
  });

  if (variants.length > 0) {
    expect(variants[0].length).toBeGreaterThan(0);
  }
});
