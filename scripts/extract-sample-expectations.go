package main

// Generates frontend/tests/playwright/fixtures/sample-manifest.json from sample-files/.
// Run: go run ./scripts/extract-sample-expectations.go

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type cellExpect struct {
	Ref   string `json:"ref"`
	Value string `json:"value"`
}

type manifestEntry struct {
	Path   string      `json:"path"`
	Editor string      `json:"editor"`
	Tier   int         `json:"tier"`
	Text   string      `json:"text,omitempty"`
	Cell   *cellExpect `json:"cell,omitempty"`
}

func main() {
	root := filepath.Join("sample-files")
	out := filepath.Join("frontend", "tests", "playwright", "fixtures", "sample-manifest.json")

	entries := []manifestEntry{
		{Path: "sample-files/sample.docx", Editor: "word", Tier: 1, Text: "Lorem ipsum"},
		{Path: "sample-files/sample.doc", Editor: "word", Tier: 1, Text: "Lorem ipsum"},
		{Path: "sample-files/sample.xlsx", Editor: "cell", Tier: 1, Cell: &cellExpect{Ref: "A1", Value: ""}},
		{Path: "sample-files/sample.xls", Editor: "cell", Tier: 1},
		{Path: "sample-files/sample.pptx", Editor: "slide", Tier: 1},
		{Path: "sample-files/sample.ppt", Editor: "slide", Tier: 1},
		{Path: "sample-files/sample.odt", Editor: "word", Tier: 2},
		{Path: "sample-files/sample.ods", Editor: "cell", Tier: 2},
		{Path: "sample-files/sample.odp", Editor: "slide", Tier: 2},
		{Path: "sample-files/sample.rtf", Editor: "word", Tier: 2},
		{Path: "sample-files/sample.txt", Editor: "word", Tier: 2, Text: "Lorem ipsum dolor sit amet"},
		{Path: "sample-files/sample.csv", Editor: "cell", Tier: 2, Cell: &cellExpect{Ref: "B2", Value: "DD37Cf93aecA6Dc"}},
		{Path: "sample-files/sample.dot", Editor: "word", Tier: 3},
		{Path: "sample-files/sample.dotx", Editor: "word", Tier: 3},
		{Path: "sample-files/sample.xlsm", Editor: "cell", Tier: 3},
		{Path: "sample-files/sample.pptm", Editor: "slide", Tier: 3},
		{Path: "sample-files/sample.pdf", Editor: "pdf", Tier: 3},
	}

	if v, err := readCSVCell(filepath.Join(root, "sample.csv"), 1, 1); err == nil && v != "" {
		for i := range entries {
			if entries[i].Path == "sample-files/sample.csv" {
				entries[i].Cell.Value = v
			}
		}
	}
	if t, err := readTxtPrefix(filepath.Join(root, "sample.txt")); err == nil {
		for i := range entries {
			if entries[i].Path == "sample-files/sample.txt" {
				entries[i].Text = t
			}
		}
	}
	if t, err := readDocxText(filepath.Join(root, "sample.docx")); err == nil {
		for i := range entries {
			if entries[i].Path == "sample-files/sample.docx" {
				entries[i].Text = firstWords(t, 3)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", out)
}

func readCSVCell(path string, row, col int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return "", err
	}
	if row >= len(records) || col >= len(records[row]) {
		return "", fmt.Errorf("out of range")
	}
	return records[row][col], nil
}

func readTxtPrefix(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(strings.Split(string(b), "\n")[0])
	return line, nil
}

func readDocxText(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
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
		s := string(b)
		s = strings.ReplaceAll(s, "<w:tab/>", " ")
		s = strings.ReplaceAll(s, "</w:p>", " ")
		var out strings.Builder
		inTag := false
		for _, ch := range s {
			if ch == '<' {
				inTag = true
				continue
			}
			if ch == '>' {
				inTag = false
				continue
			}
			if !inTag {
				out.WriteRune(ch)
			}
		}
		return strings.Join(strings.Fields(out.String()), " "), nil
	}
	return "", fmt.Errorf("document.xml missing")
}

func firstWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) < n {
		return s
	}
	return strings.Join(parts[:n], " ")
}
