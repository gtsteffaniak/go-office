import fs from "node:fs";
import path from "node:path";

/** Repo root (go-office/) relative to this file. */
export const REPO_ROOT = path.resolve(__dirname, "../../..");

export type SampleTier = 1 | 2 | 3;

export type SampleFile = {
  /** Path relative to repo root, e.g. sample-files/sample.docx */
  path: string;
  tier: SampleTier;
  editor: "word" | "cell" | "slide";
};

/**
 * Canonical sample matrix for Playwright open tests.
 * All files are git-tracked under sample-files/; CI runs make check-sample-matrix first.
 */
export const SAMPLE_FILES: SampleFile[] = [
  // Tier 1 — primary formats
  { path: "sample-files/sample.docx", tier: 1, editor: "word" },
  { path: "sample-files/sample.doc", tier: 1, editor: "word" },
  { path: "sample-files/sample.xlsx", tier: 1, editor: "cell" },
  { path: "sample-files/sample.xls", tier: 1, editor: "cell" },
  { path: "sample-files/sample.pptx", tier: 1, editor: "slide" },
  { path: "sample-files/sample.ppt", tier: 1, editor: "slide" },
  // Tier 2 — common alternates
  { path: "sample-files/sample.odt", tier: 2, editor: "word" },
  { path: "sample-files/sample.ods", tier: 2, editor: "cell" },
  { path: "sample-files/sample.odp", tier: 2, editor: "slide" },
  { path: "sample-files/sample.rtf", tier: 2, editor: "word" },
  { path: "sample-files/sample.txt", tier: 2, editor: "word" },
  { path: "sample-files/sample.csv", tier: 2, editor: "cell" },
  // Tier 3 — extended
  { path: "sample-files/sample.dot", tier: 3, editor: "word" },
  { path: "sample-files/sample.dotx", tier: 3, editor: "word" },
  { path: "sample-files/sample.xlsm", tier: 3, editor: "cell" },
  { path: "sample-files/sample.pptm", tier: 3, editor: "slide" },
  { path: "sample-files/sample.pdf", tier: 3, editor: "word" },
];

export function sampleExists(relPath: string): boolean {
  return fs.existsSync(path.join(REPO_ROOT, relPath));
}

export function samplesForTier(maxTier: SampleTier): SampleFile[] {
  return SAMPLE_FILES.filter((s) => s.tier <= maxTier);
}
