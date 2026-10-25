import { test, expect, type FrameLocator } from "../test-setup";
import { getEditorFrame, warmDemoFile, waitForEditorInteractive, waitForEditorReady } from "../editor";
import { inflateSync } from "node:zlib";

test.describe.configure({ mode: "serial" });

/** Known en_US misspelling in sample-files/sample.docx (title uses UK “calibre”). */
const SAMPLE_MISSPELLING = "calibre";

type SpellWorkerHit = { word: string; correct: boolean };

function pngRedPixelCount(png: Buffer): number {
  if (png.length < 33 || png.readUInt32BE(0) !== 0x89504e47) {
    return 0;
  }
  let pos = 8;
  let width = 0;
  let height = 0;
  let bitDepth = 8;
  let colorType = 6;
  const idat: Buffer[] = [];
  while (pos + 12 <= png.length) {
    const len = png.readUInt32BE(pos);
    const type = png.toString("ascii", pos + 4, pos + 8);
    const data = png.subarray(pos + 8, pos + 8 + len);
    if (type === "IHDR" && data.length >= 13) {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      bitDepth = data[8];
      colorType = data[9];
    } else if (type === "IDAT") {
      idat.push(data);
    } else if (type === "IEND") {
      break;
    }
    pos += 12 + len;
  }
  if (width === 0 || height === 0 || bitDepth !== 8 || (colorType !== 2 && colorType !== 6)) {
    return 0;
  }
  const raw = inflateSync(Buffer.concat(idat));
  const bytesPerPixel = colorType === 6 ? 4 : 3;
  const stride = 1 + width * bytesPerPixel;
  let red = 0;
  for (let y = 0; y < height; y++) {
    const rowStart = y * stride;
    for (let x = 0; x < width; x++) {
      const i = rowStart + 1 + x * bytesPerPixel;
      const r = raw[i];
      const g = raw[i + 1];
      const b = raw[i + 2];
      const a = colorType === 6 ? raw[i + 3] : 255;
      // Hunspell squiggles anti-alias to dark red (not always pure #ff0000).
      if (a > 40 && r >= 170 && g <= 140 && b <= 140 && r - g >= 20 && r - b >= 20) {
        red++;
      }
    }
  }
  return red;
}

async function enableSpellcheckProbe(frame: FrameLocator): Promise<void> {
  await expect
    .poll(
      async () =>
        frame.locator("body").evaluate(() => {
          const api = (window as { Asc?: { editor?: Record<string, unknown> } }).Asc?.editor;
          const worker = api?.SpellCheckApi as { worker?: { getLanguages?: () => string[] } } | undefined;
          return Boolean(
            api &&
              typeof api.SpellCheck_CallBack === "function" &&
              (worker?.worker?.getLanguages?.().length ?? 0) > 0,
          );
        }),
      { timeout: 45_000 },
    )
    .toBe(true);

  await frame.locator("body").evaluate(() => {
    const w = window as {
      Asc?: { editor?: Record<string, unknown> };
      __spellWorkerHits?: SpellWorkerHit[];
    };
    const api = w.Asc?.editor;
    if (!api || typeof api.SpellCheck_CallBack !== "function") {
      throw new Error("SpellCheck_CallBack is not available");
    }
    w.__spellWorkerHits = [];
    const record = (obj: unknown) => {
      const msg = obj as {
        type?: string;
        usrWords?: string[];
        usrCorrect?: boolean[];
      };
      if (msg?.type === "spell" && msg.usrWords && msg.usrCorrect) {
        for (let i = 0; i < msg.usrWords.length; i++) {
          w.__spellWorkerHits!.push({
            word: msg.usrWords[i],
            correct: Boolean(msg.usrCorrect[i]),
          });
        }
      }
    };
    const previous = (api.SpellCheck_CallBack as (obj: unknown) => void).bind(api);
    api.SpellCheck_CallBack = function (obj: unknown) {
      record(obj);
      return previous(obj);
    };
    (api.asc_setSpellCheck as (on: boolean) => void)(true);
    (api.asc_setDefaultLanguage as (lcid: number) => void)(1033);
    (api.asc_restartCheckSpelling as () => void).call(api);
  });
}

async function readWorkerHits(frame: FrameLocator): Promise<SpellWorkerHit[]> {
  return frame.locator("body").evaluate(() => {
    const w = window as { __spellWorkerHits?: SpellWorkerHit[] };
    return w.__spellWorkerHits ?? [];
  });
}

async function scrollToDocumentStart(frame: FrameLocator): Promise<void> {
  await frame.locator("body").click({ position: { x: 12, y: 12 }, force: true });
  await frame.locator("body").press("Control+Home");
  await frame.locator("body").evaluate(() => new Promise((r) => setTimeout(r, 800)));
}

/** Select the misspelled token so the canvas redraws squiggles in view. */
async function selectWordViaSearch(frame: FrameLocator, word: string): Promise<void> {
  await scrollToDocumentStart(frame);
  await frame.locator("body").press("Control+f");
  const searchInput = frame.locator("#search-bar-text").first();
  await searchInput.waitFor({ state: "visible", timeout: 10_000 });
  await searchInput.fill(word);
  await searchInput.press("Enter");
  await frame.locator("body").evaluate(() => new Promise((r) => setTimeout(r, 1_000)));
  await frame.locator("#search-bar-close, #search-adv-close").first().click({ timeout: 2_000 }).catch(() => {});
}

test("browser spellcheck marks misspelled words", async ({ page, request }) => {
  const dicRes = await request.get("/dictionaries/en_US/en_US.dic");
  expect(dicRes.status()).toBe(200);
  const deRes = await request.get("/dictionaries/de_DE/de_DE.dic");
  expect(deRes.status()).toBe(200);

  const gzipDic = await request.get("/dictionaries/en_US/en_US.dic", {
    headers: { "Accept-Encoding": "gzip" },
  });
  expect(gzipDic.status()).toBe(200);
  expect(gzipDic.headers()["content-encoding"] ?? "").toContain("gzip");
  const rawLen = Number(dicRes.headers()["content-length"] ?? "0");
  const gzBody = Buffer.from(await gzipDic.body());
  expect(gzBody.length).toBeGreaterThan(0);
  if (rawLen > 0) {
    expect(gzBody.length).toBeLessThan(rawLen);
  }

  const wasmRes = await request.get("/sdkjs/common/spell/spell/spell.wasm");
  expect(wasmRes.status()).toBe(200);
  expect(wasmRes.headers()["content-type"] ?? "").toContain("wasm");

  await warmDemoFile(request, "sample-files/sample.docx");
  await page.goto("/demo/view?file=sample-files%2Fsample.docx");
  await waitForEditorReady(page, "word");
  await waitForEditorInteractive(page, "word");

  const frame = getEditorFrame(page, "word");

  await expect
    .poll(
      async () =>
        frame
          .locator("body")
          .evaluate(() =>
            Boolean(
              (window as { Asc?: { editor?: { SpellCheckApi?: { worker?: unknown } } } }).Asc?.editor
                ?.SpellCheckApi?.worker,
            ),
          ),
      { timeout: 45_000 },
    )
    .toBe(true);

  await enableSpellcheckProbe(frame);

  await expect
    .poll(
      async () => {
        const hits = await readWorkerHits(frame);
        return hits.some((h) => h.word.includes(SAMPLE_MISSPELLING) && h.correct === false);
      },
      { timeout: 90_000, intervals: [500, 1000, 2000, 3000] },
    )
    .toBe(true);

  await frame.locator("body").evaluate(() => new Promise((r) => setTimeout(r, 2_000)));
  await selectWordViaSearch(frame, SAMPLE_MISSPELLING);

  const editor = frame.locator("#editor_sdk, #id_view, #id_main").first();
  const screenshotPath = "test-results/spellcheck-underline.png";
  const screenshot = await editor.screenshot({ path: screenshotPath });
  const redUnderlinePixels = pngRedPixelCount(screenshot);
  expect(
    redUnderlinePixels,
    `expected red spellcheck underline pixels in ${screenshotPath} (got ${redUnderlinePixels})`,
  ).toBeGreaterThan(8);
});
