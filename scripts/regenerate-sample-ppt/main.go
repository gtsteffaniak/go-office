//go:build linux

package main

// Regenerate sample-files/sample.ppt from sample.pptx via x2t (or OOXML fallback copy).
// Run from repo root: go run ./scripts/regenerate-sample-ppt

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

const (
	pptxRel = "sample-files/sample.pptx"
	pptRel  = "sample-files/sample.ppt"
	marker  = "My Presentation"
	slide   = "ppt/slides/slide1.xml"
)

func main() {
	repo, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo root: %v\n", err)
		os.Exit(1)
	}

	assets := office.AssetDirFromEnv(filepath.Join(repo, "assets"))
	x2t := filepath.Join(assets, "converter", "bin", "x2t")
	if st, statErr := os.Stat(x2t); statErr != nil || st.IsDir() {
		fmt.Fprintf(os.Stderr, "x2t not found at %s (set OFFICE_ASSETS or fetch assets)\n", x2t)
		os.Exit(1)
	}

	pptxPath := filepath.Join(repo, pptxRel)
	if _, statErr := os.Stat(pptxPath); statErr != nil {
		fmt.Fprintf(os.Stderr, "missing %s: %v\n", pptxRel, statErr)
		os.Exit(1)
	}

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert.New: %v\n", err)
		os.Exit(1)
	}

	outDir, err := os.MkdirTemp("", "regen-ppt-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(outDir)

	outPath := filepath.Join(outDir, "sample.ppt")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("converting %s → %s\n", pptxRel, pptRel)
	convErr := conv.ConvertOffice(ctx, pptxPath, outPath, "pptx", "ppt")
	if convErr != nil {
		fmt.Fprintf(os.Stderr, "x2t pptx→ppt failed (%v); using pptx OOXML fallback at .ppt path\n", convErr)
		data, readErr := os.ReadFile(pptxPath)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "read pptx: %v\n", readErr)
			os.Exit(1)
		}
		if writeErr := os.WriteFile(outPath, data, 0o644); writeErr != nil {
			fmt.Fprintf(os.Stderr, "write fallback: %v\n", writeErr)
			os.Exit(1)
		}
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read output: %v\n", err)
		os.Exit(1)
	}
	xml, err := readZipEntryFile(outPath, slide)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", slide, err)
		os.Exit(1)
	}
	if !strings.Contains(xml, marker) {
		fmt.Fprintf(os.Stderr, "marker %q not found in %s\n", marker, slide)
		os.Exit(1)
	}

	dest := filepath.Join(repo, pptRel)
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", pptRel, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes), verified %s contains %q\n", pptRel, len(data), slide, marker)
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

func readZipEntryFile(path, name string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != name && !strings.HasSuffix(f.Name, "/"+name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", fmt.Errorf("%s not found in %s", name, path)
}
