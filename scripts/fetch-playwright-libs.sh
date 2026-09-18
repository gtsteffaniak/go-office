#!/bin/sh
# Provide the system shared libraries Chromium needs, without root.
#
# Playwright's Chromium requires libnspr4, libnss3 and libasound.so.2. On a minimal host
# (container, CI image, or a read-only system tree) these may be absent and not installable,
# which shows up as:
#
#   chrome-headless-shell: error while loading shared libraries: libnspr4.so: cannot open
#   shared object file: No such file or directory
#
# This script downloads the .deb packages, extracts them into .pw-libs/, and prints the
# LD_LIBRARY_PATH to use. It does not modify the system.
#
# Usage:
#   scripts/fetch-playwright-libs.sh
#   LD_LIBRARY_PATH="$PWD/.pw-libs/root/usr/lib/x86_64-linux-gnu" npx playwright test ...
set -eu

repo="$(cd "$(dirname "$0")/.." && pwd)"
out="$repo/.pw-libs"
libdir="$out/root/usr/lib/x86_64-linux-gnu"
arch="$(dpkg --print-architecture 2>/dev/null || echo amd64)"

mkdir -p "$out"
cd "$out"

# libasound2 was renamed libasound2t64 on newer Ubuntu; try both and tolerate one failing.
for pkg in libnspr4 libnss3 libasound2t64 libasound2; do
	if ! apt-get download "$pkg" >/dev/null 2>&1; then
		echo "note: could not download $pkg (may be unnecessary on this host)" >&2
	fi
done

if ! ls ./*.deb >/dev/null 2>&1; then
	echo "error: no packages downloaded; install libnspr4/libnss3/libasound2 with your package manager instead" >&2
	exit 1
fi

for deb in ./*.deb; do
	dpkg-deb -x "$deb" "$out/root" 2>/dev/null || {
		echo "error: cannot extract $deb" >&2
		exit 1
	}
done

# Some packages ship only the versioned soname; add the linker name where needed.
[ -f "$libdir/libasound.so.2.0.0" ] && [ ! -e "$libdir/libasound.so.2" ] &&
	ln -sf libasound.so.2.0.0 "$libdir/libasound.so.2"

echo "libraries extracted to: $libdir"
echo
echo "run Playwright with:"
echo "  LD_LIBRARY_PATH=\"$libdir\" npx playwright test"
