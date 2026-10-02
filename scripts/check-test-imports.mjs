#!/usr/bin/env node
/**
 * Fail when a Playwright spec calls an editor helper it did not import.
 *
 * `npx playwright test --list` transpiles without full type checking, so an undefined
 * reference only surfaces at runtime — which is how a removed import shipped and produced:
 *
 *   ReferenceError: waitForPersistedMarker is not defined
 *     at post-save-stability.spec.ts:52
 *
 * This script compares the helper names a spec references against the names it imports
 * (and against its own local declarations), catching that class of error before CI.
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const testRoot = path.join(repo, "frontend", "tests", "playwright");

/** Exported names from a TypeScript module, via `export function|const|type|class`. */
function exportedNames(source) {
  const names = new Set();
  const patterns = [
    /export\s+(?:async\s+)?function\s+([A-Za-z_$][\w$]*)/g,
    /export\s+const\s+([A-Za-z_$][\w$]*)/g,
    /export\s+(?:type|interface|class|enum)\s+([A-Za-z_$][\w$]*)/g,
  ];
  for (const re of patterns) {
    for (const m of source.matchAll(re)) names.add(m[1]);
  }
  return names;
}

function importedNames(source) {
  const names = new Set();
  for (const m of source.matchAll(/import\s*\{([^}]*)\}\s*from/g)) {
    for (const part of m[1].split(",")) {
      const name = part.trim().split(/\s+as\s+/).pop().trim();
      if (name) names.add(name);
    }
  }
  for (const m of source.matchAll(/import\s+([A-Za-z_$][\w$]*)\s+from/g)) names.add(m[1]);
  return names;
}

function declaredNames(source) {
  const names = new Set();
  for (const m of source.matchAll(/(?:function|const|let|class)\s+([A-Za-z_$][\w$]*)/g)) {
    names.add(m[1]);
  }
  return names;
}

function walk(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) return walk(full);
    return e.name.endsWith(".ts") ? [full] : [];
  });
}

const editorPath = path.join(testRoot, "editor.ts");
const helpers = exportedNames(fs.readFileSync(editorPath, "utf8"));
const files = walk(testRoot).filter((f) => f.endsWith(".spec.ts"));

const problems = [];
for (const file of files) {
  const source = fs.readFileSync(file, "utf8");
  const imported = importedNames(source);
  const declared = declaredNames(source);
  for (const helper of helpers) {
    // Referenced as a bare call, not a property access or a definition.
    const callRe = new RegExp(`(?<![.\\w$])${helper}\\s*\\(`, "g");
    if (!callRe.test(source)) continue;
    if (imported.has(helper) || declared.has(helper)) continue;
    problems.push(`${path.relative(repo, file)}: calls ${helper}() but does not import it`);
  }
}

if (problems.length > 0) {
  console.error("error: specs reference editor helpers they do not import:");
  for (const p of problems) console.error(`  ${p}`);
  process.exit(1);
}
console.log(`test imports ok (${files.length} specs checked against ${helpers.size} editor exports)`);
