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

echo "sample matrix ok ($dir, $(echo $required | wc -w | tr -d ' ') files)"
