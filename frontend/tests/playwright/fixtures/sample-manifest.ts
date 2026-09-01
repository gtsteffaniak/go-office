import fs from "node:fs";
import path from "node:path";

export type CellExpectation = {
  ref: string;
  value: string;
};

export type SampleManifestEntry = {
  path: string;
  editor: "word" | "cell" | "slide" | "pdf";
  tier: 1 | 2 | 3;
  text?: string;
  cell?: CellExpectation;
};

export const SAMPLE_MANIFEST = JSON.parse(
  fs.readFileSync(path.join(__dirname, "sample-manifest.json"), "utf8"),
) as SampleManifestEntry[];

export function manifestForTier(maxTier: 1 | 2 | 3): SampleManifestEntry[] {
  return SAMPLE_MANIFEST.filter((e) => e.tier <= maxTier);
}

export function manifestEntry(path: string): SampleManifestEntry | undefined {
  return SAMPLE_MANIFEST.find((e) => e.path === path);
}
