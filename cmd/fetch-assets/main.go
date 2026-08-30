//go:build linux

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/quantumx-apps/go-office/internal/assetfetch"
)

func main() {
	force := flag.Bool("force", false, "re-download even if assets are present")
	out := flag.String("out", "", "output directory (default: ./assets)")
	versionFile := flag.String("version-file", "", "path to scripts/euro-office.version")
	flag.Parse()

	root, err := assetfetch.FindRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	outDir := *out
	if outDir == "" {
		outDir = filepath.Join(root, "assets")
	}
	vf := *versionFile
	if vf == "" {
		vf = filepath.Join(root, "scripts", "euro-office.version")
	}

	if err := assetfetch.Fetch(assetfetch.Options{
		OutDir:      outDir,
		VersionFile: vf,
		Force:       *force,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "fetch-assets:", err)
		os.Exit(1)
	}
}
