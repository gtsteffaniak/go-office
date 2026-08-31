import fs from "node:fs";
import path from "node:path";
import type { TestInfo } from "@playwright/test";
import { REPO_ROOT } from "./samples";

/**
 * Copy a git-tracked sample into a per-test writable path so parallel Playwright
 * workers never mutate the canonical sample-files/* originals.
 */
export function forkSample(relPath: string, info: TestInfo): string {
  const normalized = relPath.replace(/\\/g, "/");
  const base = path.posix.basename(normalized);
  const dirName = `${info.workerIndex}-${info.parallelIndex}-${info.testId}`;
  const destRel = `sample-files/playwright/${dirName}/${base}`;
  const srcAbs = path.join(REPO_ROOT, normalized);
  const destAbs = path.join(REPO_ROOT, destRel);
  fs.mkdirSync(path.dirname(destAbs), { recursive: true });
  fs.copyFileSync(srcAbs, destAbs);
  return destRel;
}
