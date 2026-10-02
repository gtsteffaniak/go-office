#!/usr/bin/env node
/**
 * Pre-convert canonical sample-files before parallel Playwright open tests.
 * Uses shared document keys (path-based) so workers reuse Editor.bin cache.
 * Also pre-generates demo landing thumbnails so they do not contend with x2t during tests.
 *
 * Fails fast: global deadline (default 60s) and per-request timeout (default 20s).
 */
import fs from "node:fs";
import path from "node:path";

const base = (process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8080").replace(/\/$/, "");
const demoBase = (process.env.PLAYWRIGHT_DEMO_BASE ?? "/demo").replace(/\/$/, "");
const apiBase = (process.env.PLAYWRIGHT_API_BASE ?? "/api/office").replace(/\/$/, "");
const samplesDir = process.env.PLAYWRIGHT_SAMPLES_DIR ?? "/app/sample-files";
const convertLimit = Math.max(1, Number(process.env.OFFICE_CONVERT_LIMIT ?? 6));
const deadlineMs = Math.max(1_000, Number(process.env.PLAYWRIGHT_PREWARM_DEADLINE_MS ?? 60_000));
const requestTimeoutMs = Math.max(1_000, Number(process.env.PLAYWRIGHT_PREWARM_REQUEST_MS ?? 20_000));
const prewarmThumbnails = process.env.PLAYWRIGHT_PREWARM_THUMBNAILS !== "0";

const startedAt = Date.now();
let completed = 0;
let inFlight = "";

function elapsedSec() {
  return ((Date.now() - startedAt) / 1000).toFixed(1);
}

function assertWithinDeadline(phase) {
  const elapsed = Date.now() - startedAt;
  if (elapsed > deadlineMs) {
    process.stderr.write(
      `pre-warm deadline exceeded after ${(elapsed / 1000).toFixed(1)}s (limit=${deadlineMs}ms) ` +
        `phase=${phase} completed=${completed} inFlight=${inFlight || "none"}\n`,
    );
    process.exit(1);
  }
}

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

async function fetchWithTimeout(url) {
  return fetch(url, {
    cache: "no-store",
    signal: AbortSignal.timeout(requestTimeoutMs),
  });
}

async function warmFile(relPath) {
  const url = `${base}${demoBase}/warm?file=${encodeURIComponent(relPath)}`;
  const res = await fetchWithTimeout(url);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`warm ${relPath}: HTTP ${res.status} ${body}`);
  }
}

async function warmThumbnail(relPath) {
  const url = `${base}${apiBase}/demo/thumbnail?file=${encodeURIComponent(relPath)}`;
  const res = await fetchWithTimeout(url);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`thumbnail ${relPath}: HTTP ${res.status} ${body}`);
  }
}

async function runPool(files, worker, phase) {
  let next = 0;
  const total = files.length;
  const workers = Array.from({ length: Math.min(convertLimit, files.length) }, async () => {
    while (next < files.length) {
      assertWithinDeadline(phase);
      const index = next++;
      const file = files[index];
      inFlight = file;
      try {
        await worker(file);
        completed += 1;
        process.stderr.write(`${phase} ${completed}/${total} ${file} (${elapsedSec()}s elapsed)\n`);
      } finally {
        if (inFlight === file) {
          inFlight = "";
        }
      }
    }
  });
  await Promise.all(workers);
}

const files = listSamples();
if (files.length === 0) {
  process.exit(0);
}

process.stderr.write(
  `pre-warming ${files.length} sample(s) (limit=${convertLimit}, deadline=${deadlineMs}ms, ` +
    `requestTimeout=${requestTimeoutMs}ms)…\n`,
);

await runPool(files, warmFile, "warm");
process.stderr.write(`sample pre-warm complete (${elapsedSec()}s)\n`);

if (prewarmThumbnails) {
  assertWithinDeadline("thumbnails-start");
  process.stderr.write(`pre-warming ${files.length} thumbnail(s) (limit=${convertLimit})…\n`);
  await runPool(files, warmThumbnail, "warmThumbnail");
  process.stderr.write(`thumbnail pre-warm complete (${elapsedSec()}s)\n`);
} else {
  process.stderr.write("thumbnail pre-warm skipped (PLAYWRIGHT_PREWARM_THUMBNAILS=0)\n");
}
