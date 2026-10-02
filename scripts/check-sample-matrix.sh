#!/bin/sh
# Verify the full Playwright sample matrix exists under sample-files/.
set -eu

dir="${1:-sample-files}"
dir="${dir%/}"

required="
sample.docx sample.doc sample.xlsx sample.xls sample.pptx sample.ppt
sample.odt sample.ods sample.odp sample.rtf sample.txt sample.csv
sample.dot sample.dotx sample.xlsm sample.pptm sample.pdf
"

missing=""
for name in $required; do
	if [ ! -f "$dir/$name" ]; then
		missing="$missing
  $dir/$name"
	fi
done

if [ -n "$missing" ]; then
	echo "error: sample matrix incomplete — missing:$missing" >&2
	echo "       See sample-files/README.md" >&2
	exit 1
fi

# Fail fast on corrupt ODF/OOXML zip payloads (deflated entries must decompress).
check_zip_entry() {
	file="$1"
	entry="$2"
	if ! python3 - "$dir/$file" "$entry" <<'PY'
import sys, zipfile
path, entry = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(path) as zf:
    try:
        data = zf.read(entry)
    except KeyError:
        print(f"error: {path}: missing zip entry {entry}", file=sys.stderr)
        sys.exit(1)
    except Exception as exc:
        print(f"error: {path}: cannot read {entry}: {exc}", file=sys.stderr)
        sys.exit(1)
if not data:
    print(f"error: {path}: {entry} is empty", file=sys.stderr)
    sys.exit(1)
PY
	then
		echo "error: zip integrity check failed for $dir/$file ($entry)" >&2
		exit 1
	fi
}

check_zip_entry sample.docx word/document.xml
check_zip_entry sample.xlsx xl/sharedStrings.xml
check_zip_entry sample.pptx ppt/slides/slide1.xml
check_zip_entry sample.odt content.xml
check_zip_entry sample.ods content.xml
check_zip_entry sample.odp content.xml

# Format integrity: each fixture must actually be the format its extension claims.
#
# The save path can legitimately persist OOXML bridge bytes at a legacy path
# (assemblyFormatAsOrigin rollback) for formats x2t cannot write (.xls/.doc/.ppt). A save
# that writes such bytes into the *tracked* sample tree silently replaces a real binary
# fixture with an OOXML one, and every later test then "passes" against the wrong format.
# This check makes that corruption fail the build instead.
check_legacy_binary() {
	file="$1"
	if ! python3 - "$dir/$file" <<'PY'
import sys
path = sys.argv[1]
with open(path, "rb") as f:
    head = f.read(8)
ole2 = bytes.fromhex("d0cf11e0a1b11ae1")
if head.startswith(b"PK"):
    print(
        f"error: {path} is a ZIP/OOXML file but has a legacy binary extension. "
        "A save (assemblyFormatAsOrigin rollback) has overwritten the tracked fixture; "
        "restore a genuine binary original.",
        file=sys.stderr,
    )
    sys.exit(1)
if not head.startswith(ole2):
    print(f"error: {path} is not a valid OLE2 compound file (magic={head.hex()})", file=sys.stderr)
    sys.exit(1)
PY
	then
		echo "error: legacy binary format check failed for $dir/$file" >&2
		exit 1
	fi
}

# OLE2 legacy formats must be genuine compound documents.
# x2t cannot write any of these (xls exit 88, doc exit 80, ppt exit 88), so a real binary
# original must be kept in the tree; a save would replace it with an OOXML package.
for legacy in sample.xls sample.doc sample.ppt; do
	check_legacy_binary "$legacy"
done

# OOXML/ODF formats must be ZIP packages (not, say, an OLE2 file).
check_zip_container() {
	file="$1"
	if ! python3 - "$dir/$file" <<'PY'
import sys
path = sys.argv[1]
with open(path, "rb") as f:
    head = f.read(4)
if not head.startswith(b"PK"):
    print(f"error: {path} must be a ZIP package (magic={head.hex()})", file=sys.stderr)
    sys.exit(1)
PY
	then
		echo "error: zip container check failed for $dir/$file" >&2
		exit 1
	fi
}

for zipped in sample.docx sample.xlsx sample.odt sample.ods sample.pptx sample.xlsm sample.pptm \
	sample.dotx sample.odp; do
	check_zip_container "$zipped"
done

# .xls is a genuine binary workbook, so no OOXML "xl/workbook.xml" entry may be present.
# This is the check that catches a save writing rollback bytes over the tracked fixture.
if ! python3 - "$dir/sample.xls" <<'PY'
import sys, zipfile
try:
    with zipfile.ZipFile(sys.argv[1]) as zf:
        names = zf.namelist()
except zipfile.BadZipFile:
    sys.exit(0)  # genuine OLE2: not a zip, which is what we want
if any(n.endswith("xl/workbook.xml") for n in names):
    print("error: sample.xls contains OOXML parts (xl/workbook.xml)", file=sys.stderr)
    sys.exit(1)
PY
then
	echo "error: sample.xls OOXML-leak check failed" >&2
	exit 1
fi

# ODS B2 must match CSV row 2 Customer Id (Playwright save.spec.ts).
if ! python3 - "$dir/sample.ods" <<'PY'
import sys, zipfile
marker = "DD37Cf93aecA6Dc"
with zipfile.ZipFile(sys.argv[1]) as zf:
    xml = zf.read("content.xml").decode("utf-8", "replace")
if marker not in xml:
    print(f"error: sample.ods content.xml missing marker {marker}", file=sys.stderr)
    sys.exit(1)
PY
then
	echo "error: sample.ods marker check failed" >&2
	exit 1
fi

# PPT slide title for Playwright save.spec.ts (pptx XML + binary ppt bytes).
ppt_marker="My Presentation"
if ! python3 - "$dir/sample.pptx" <<'PY'
import sys, zipfile, re
marker = "My Presentation"
with zipfile.ZipFile(sys.argv[1]) as zf:
    xml = zf.read("ppt/slides/slide1.xml").decode("utf-8", "replace")
if marker not in xml:
    print(f"error: sample.pptx slide1.xml missing marker {marker}", file=sys.stderr)
    sys.exit(1)
PY
then
	echo "error: sample.pptx marker check failed" >&2
	exit 1
fi

# sample.ppt must be a genuine binary deck AND contain the slide text the tests assert.
# PowerPoint stores slide text in TextBytesAtom (0x0FA8) records, or UTF-16LE
# (TextCharsAtom, 0x0FA0) for non-Latin text, so check both encodings.
if ! python3 - "$dir/sample.ppt" "$ppt_marker" <<'PY'
import sys
path, marker = sys.argv[1], sys.argv[2]
with open(path, "rb") as f:
    data = f.read()
if data[:2] == b"PK":
    print(
        f"error: {path} is an OOXML package; a genuine binary .ppt is required "
        "(a save rollback has overwritten the tracked fixture)",
        file=sys.stderr,
    )
    sys.exit(1)
if marker.encode("latin-1") not in data and marker.encode("utf-16le") not in data:
    print(f"error: {path} missing decoded marker {marker!r}", file=sys.stderr)
    sys.exit(1)
PY
then
	echo "error: sample.ppt marker check failed" >&2
	exit 1
fi

# RTF body text may be \\uc1\\uNN* unicode runs; decode before marker checks.
if ! python3 - "$dir/sample.rtf" <<'PY'
import re, sys

def rtf_plain(rtf: str) -> str:
    out: list[str] = []
    i = 0
    uc_skip = 1
    while i < len(rtf):
        if rtf.startswith("\\par", i):
            out.append("\n")
            i += 4
            if i < len(rtf) and rtf[i] == " ":
                i += 1
            continue
        if rtf.startswith("\\uc", i):
            j = i + 3
            start = j
            while j < len(rtf) and rtf[j].isdigit():
                j += 1
            if j > start:
                uc_skip = int(rtf[start:j])
            if j < len(rtf) and rtf[j] == " ":
                j += 1
            i = j
            continue
        m = re.match(r"\\u(-?\d+)", rtf[i:])
        if m:
            cp = int(m.group(1))
            if cp < 0:
                cp += 65536
            out.append(chr(cp) if cp < 0x110000 else "?")
            j = i + len(m.group(0))
            if j < len(rtf) and rtf[j] == "?":
                j += 1
            for _ in range(uc_skip):
                if j < len(rtf):
                    j += 1
            i = j
            continue
        if rtf[i] == "\\":
            j = i + 1
            if j < len(rtf) and rtf[j] == "*":
                j += 1
            while j < len(rtf) and rtf[j].isalpha():
                j += 1
            i = j
            continue
        if rtf[i] in "{}":
            i += 1
            continue
        out.append(rtf[i])
        i += 1
    return "".join(out)

path = sys.argv[1]
with open(path, "rb") as f:
    raw = f.read().decode("latin-1", errors="replace")
text = rtf_plain(raw)
for marker in ("SYSTEM BRIEF", "DAILY LOG", "Reminder"):
    if marker not in text:
        print(f"error: {path} missing decoded marker {marker!r}", file=sys.stderr)
        sys.exit(1)
PY
then
	echo "error: sample.rtf marker check failed" >&2
	exit 1
fi

echo "sample matrix ok ($dir, $(echo $required | wc -w | tr -d ' ') files)"
