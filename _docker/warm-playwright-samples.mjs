#!/usr/bin/env node
/**
 * Pre-convert canonical sample-files before parallel Playwright open tests.
 * Uses shared document keys (path-based) so workers reuse Editor.bin cache.
 */
import fs from "node:fs";
import path from "node:path";

const base = (process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080").replace(/\/$/, "");
const demoBase = (process.env.PLAYWRIGHT_DEMO_BASE ?? "/demo").replace(/\/$/, "");
const samplesDir = process.env.PLAYWRIGHT_SAMPLES_DIR ?? "/app/sample-files";
const convertLimit = Math.max(1, Number(process.env.OFFICE_CONVERT_LIMIT ?? 4));

const OPEN_EXT =
  /\.(docx?|dotx?|xlsx?|xlsm|ods|csv|pptx?|pptm|odp|odt|rtf|txt|pdf)$/i;

function listSamples() {
  if (!fs.existsSync(samplesDir)) {
    console.warn(`warm: samples dir missing: ${samplesDir}`);
    return [];
  }
  return fs
    .readdirSync(samplesDir)
    .filter((name) => OPEN_EXT.test(name))
    .map((name) => `sample-files/${name}`)
    .sort();
}

async function warmFile(relPath) {
  const url = `${base}${demoBase}/warm?file=${encodeURIComponent(relPath)}`;
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`warm ${relPath}: HTTP ${res.status} ${body}`);
  }
}

async function runPool(files) {
  let next = 0;
  const workers = Array.from({ length: Math.min(convertLimit, files.length) }, async () => {
    while (next < files.length) {
      const file = files[next++];
      process.stderr.write(`warming ${file}\n`);
      await warmFile(file);
    }
  });
  await Promise.all(workers);
}

const files = listSamples();
if (files.length === 0) {
  process.exit(0);
}

process.stderr.write(`pre-warming ${files.length} sample(s) (limit=${convertLimit})…\n`);
await runPool(files);
process.stderr.write("sample pre-warm complete\n");
