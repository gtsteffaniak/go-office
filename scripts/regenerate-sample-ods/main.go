//go:build linux

package main

// Regenerate sample-files/sample.ods from sample.csv via x2t.
// Run from repo root: go run ./scripts/regenerate-sample-ods

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
	csvRel   = "sample-files/sample.csv"
	odsRel   = "sample-files/sample.ods"
	marker   = "DD37Cf93aecA6Dc"
	xmlEntry = "content.xml"
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

	csvPath := filepath.Join(repo, csvRel)
	if _, statErr := os.Stat(csvPath); statErr != nil {
		fmt.Fprintf(os.Stderr, "missing %s: %v\n", csvRel, statErr)
		os.Exit(1)
	}

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert.New: %v\n", err)
		os.Exit(1)
	}

	outDir, err := os.MkdirTemp("", "regen-ods-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(outDir)

	outPath := filepath.Join(outDir, "sample.ods")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("converting %s → %s\n", csvRel, odsRel)
	if convErr := conv.ConvertOffice(ctx, csvPath, outPath, "csv", "ods"); convErr != nil {
		fmt.Fprintf(os.Stderr, "ConvertOffice: %v\n", convErr)
		os.Exit(1)
	}

	xml, err := readZipEntry(outPath, xmlEntry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", xmlEntry, err)
		os.Exit(1)
	}
	if !strings.Contains(xml, marker) {
		fmt.Fprintf(os.Stderr, "marker %q not found in %s\n", marker, xmlEntry)
		os.Exit(1)
	}

	dest := filepath.Join(repo, odsRel)
	data, err := os.ReadFile(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read output: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", odsRel, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes), verified %s contains %q\n", odsRel, len(data), xmlEntry, marker)
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

func readZipEntry(path, name string) (string, error) {
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
