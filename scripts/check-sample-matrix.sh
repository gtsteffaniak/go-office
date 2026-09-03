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

if ! python3 - "$dir/sample.ppt" "$ppt_marker" <<'PY'
import sys, zipfile
path, marker = sys.argv[1], sys.argv[2]
with open(path, "rb") as f:
    head = f.read(2)
if head == b"PK":
    with zipfile.ZipFile(path) as zf:
        xml = zf.read("ppt/slides/slide1.xml").decode("utf-8", "replace")
    if marker not in xml:
        print(f"error: {path} slide1.xml missing marker {marker}", file=sys.stderr)
        sys.exit(1)
else:
    with open(path, "rb") as f:
        data = f.read()
    if marker.encode("utf-8") not in data:
        print(f"error: {path} missing marker {marker}", file=sys.stderr)
        sys.exit(1)
PY
then
	echo "error: sample.ppt marker check failed" >&2
	exit 1
fi

echo "sample matrix ok ($dir, $(echo $required | wc -w | tr -d ' ') files)"
