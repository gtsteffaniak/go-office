# Sample documents for demo and Playwright E2E tests

Naming: `sample-files/sample.{ext}`

Tier 1 (required for strict CI — set `PLAYWRIGHT_STRICT=1`):
`sample.docx` `sample.doc` `sample.xlsx` `sample.xls` `sample.pptx` `sample.ppt`

Tier 2:
`sample.odt` `sample.ods` `sample.odp` `sample.rtf` `sample.txt` `sample.csv`

Tier 3:
`sample.dot` `sample.dotx` `sample.xlsm` `sample.pptm` `sample.pdf`

Sources: https://filesamples.com https://file-examples.com
Keep files small (<500 KB) for CI.

## Fixture format integrity (important)

`make check-sample-matrix` verifies each fixture is actually the format its extension
claims. This is not cosmetic — the save path can write OOXML bridge bytes at a legacy path
(`assemblyFormatAsOrigin` rollback), and if that happens to a **tracked** sample it silently
replaces a real binary fixture with an OOXML one. Every later test then passes against the
wrong format.

This has happened in this repo: `sample.xls` and `sample.doc` were replaced with OOXML
packages (ZIP magic `PK`) by a save originating from a run whose output path pointed into
`sample-files/`. Both were restored to genuine OLE2 compound files (`D0CF11E0A1B11AE1`).

Rules:

1. **Never write a save output into `sample-files/`.** Playwright forks samples into
   `sample-files/playwright/` (gitignored) via `forkSample`; keep it that way. If a run
   modifies a tracked sample, `git status sample-files/` will show it — treat that as a bug.
2. **Legacy binaries must stay binary.** `sample.xls` and `sample.doc` must remain OLE2
   compound files. Verify with `file sample-files/sample.xls` → "Composite Document File".
3. **`sample.ppt` is a documented exception.** x2t cannot write binary PowerPoint
   (verified: `pptx→ppt` exits 88), so no tooling in this repo can author a `.ppt`
   containing the text the slide tests assert (`My Presentation`). The tracked `sample.ppt`
   is therefore an OOXML package at a `.ppt` path — the same shape the server's assembly
   rollback produces. The check script lists this exemption explicitly.
4. **x2t cannot write any legacy binary format.** Verified against the bundled converter:
   `xlsx→xls` exit 88, `docx→doc` exit 80, `pptx→ppt` exit 88. Do not "fix" a save failure
   by regenerating these fixtures with x2t; it will produce OOXML bytes.
