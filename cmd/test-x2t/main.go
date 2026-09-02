//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/quantumx-apps/go-office/internal/convert"
)

func main() {
	assets := flag.String("assets", "assets", "Euro-Office assets directory")
	sample := flag.String("sample", "sample-files/sample.doc", "document to convert")
	flag.Parse()

	x2t := filepath.Join(*assets, "converter", "bin", "x2t")
	if st, err := os.Stat(x2t); err != nil {
		fmt.Fprintf(os.Stderr, "x2t missing: %s (%v)\n", x2t, err)
		os.Exit(1)
	} else {
		fmt.Printf("x2t: %s mode=%s size=%d\n", x2t, st.Mode().Perm(), st.Size())
	}

	conv, err := convert.New(convert.Options{AssetDir: *assets})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	outDir, err := os.MkdirTemp("", "go-office-test-x2t-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(outDir)

	ctx := context.Background()
	if err = conv.ToEditorBin(ctx, *sample, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "convert failed: %v\n", err)
		os.Exit(1)
	}

	outFile := filepath.Join(outDir, "Editor.bin")
	st, err := os.Stat(outFile)
	if err != nil || st.Size() == 0 {
		fmt.Fprintf(os.Stderr, "Editor.bin missing or empty in %s\n", outDir)
		os.Exit(1)
	}
	fmt.Printf("ok: Editor.bin bytes=%d\n", st.Size())
}
