import { test, expect } from "../test-setup";
import { getEditorFrame, warmDemoFile, waitForEditorReady } from "../editor";

test.describe.configure({ mode: "serial" });

test("browser spellcheck loads dictionaries and wasm", async ({ page, request }) => {
  const dicRes = await request.get("/dictionaries/en_US/en_US.dic");
  expect(dicRes.status()).toBe(200);
  const deRes = await request.get("/dictionaries/de_DE/de_DE.dic");
  expect(deRes.status()).toBe(200);

  const wasmPromise = page.waitForResponse(
    (res) => res.url().includes("/sdkjs/common/spell/spell/spell.wasm") && res.status() === 200,
    { timeout: 60_000 },
  );

  await warmDemoFile(request, "sample-files/sample.docx");
  await page.goto("/demo/view?file=sample-files%2Fsample.docx");
  await waitForEditorReady(page, "word");
  await wasmPromise;

  const frame = getEditorFrame(page, "word");
  const variants = await frame.locator("body").evaluate(async () => {
    const w = window as {
      Asc?: { editor?: Record<string, unknown> };
    };
    const api = w.Asc?.editor;
    if (!api) {
      return { ok: false, reason: "no editor api" };
    }
    return await new Promise<{ ok: boolean; reason?: string; variants?: string[] }>((resolve) => {
      const timeout = window.setTimeout(() => resolve({ ok: false, reason: "timeout" }), 45_000);
      api.asc_onSpellCheckVariantsFound = (_props: unknown, list: string[]) => {
        if (list && list.length > 0) {
          window.clearTimeout(timeout);
          resolve({ ok: true, variants: list.slice(0, 5) });
        }
      };
      const insert = api.asc_insertText ?? api.asc_AddText ?? api.asc_enterText;
      if (typeof insert !== "function") {
        window.clearTimeout(timeout);
        resolve({ ok: false, reason: "no insert api" });
        return;
      }
      (insert as (text: string) => void)("zzqwxteh");
      const restart = api.asc_restartCheckSpelling;
      if (typeof restart === "function") {
        (restart as () => void)();
      }
    });
  });

  expect(variants.ok, variants.reason ?? "spellcheck variants").toBeTruthy();
  if (variants.variants?.length) {
    expect(variants.variants[0].length).toBeGreaterThan(0);
  }

  await page.screenshot({
    path: "test-results/spellcheck-underline.png",
    fullPage: true,
  });
});
