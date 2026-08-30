//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/quantumx-apps/go-office/internal/assetfetch"
	"github.com/quantumx-apps/go-office/internal/convert"
)

func main() {
	assets := flag.String("assets", "assets", "Euro-Office assets directory")
	sample := flag.String("sample", "sample-files/sample.doc", "document to convert")
	report := flag.String("report", "doctor-report.txt", "human-readable report path")
	flag.Parse()

	lines := []string{"go-office doctor", time.Now().Format(time.RFC3339), ""}
	log := func(msg string, data map[string]any) {
		line := msg
		if len(data) > 0 {
			line += ": " + fmt.Sprint(data)
		}
		lines = append(lines, line)
		fmt.Println(line)
	}

	x2t := filepath.Join(*assets, "converter", "bin", "x2t")
	if st, err := os.Stat(x2t); err != nil {
		log("x2t stat failed", map[string]any{"path": x2t, "err": err.Error()})
		writeReport(*report, lines)
		os.Exit(1)
	}
	log("x2t stat", map[string]any{
		"path": x2t, "perm": st.Mode().Perm().String(), "executable": st.Mode()&0o111 != 0, "size": st.Size(),
	})

	for _, path := range []string{
		filepath.Join(*assets, "sdkjs", "common", "AllFonts.js"),
		filepath.Join(*assets, "converter", "bin", "font_selection.bin"),
		filepath.Join(*assets, "converter", "bin", "AllFonts.js"),
	} {
		if st, err := os.Stat(path); err != nil {
			log("font file missing", map[string]any{"path": path, "err": err.Error()})
		} else {
			log("font file", map[string]any{"path": path, "bytes": st.Size()})
		}
	}
	if !assetfetch.FontsReady(*assets) {
		log("fonts not ready", map[string]any{"hint": "run: make fonts"})
	}

	binDir := filepath.Join(*assets, "converter", "bin")
	cmd := exec.Command("./x2t")
	cmd.Dir = binDir
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+binDir)
	banner, err := cmd.CombinedOutput()
	log("x2t banner", map[string]any{"err": fmt.Sprint(err), "out": strings.TrimSpace(string(banner))})

	conv, err := convert.New(convert.Options{AssetDir: *assets})
	if err != nil {
		log("convert.New failed", map[string]any{"err": err.Error()})
		writeReport(*report, lines)
		os.Exit(1)
	}
	log("convert.New ok", nil)

	outDir, err := os.MkdirTemp("", "go-office-doctor-*")
	if err != nil {
		log("temp dir failed", map[string]any{"err": err.Error()})
		writeReport(*report, lines)
		os.Exit(1)
	}
	defer os.RemoveAll(outDir)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := conv.ToEditorBin(ctx, *sample, outDir); err != nil {
		log("ToEditorBin failed", map[string]any{"err": err.Error()})
		writeReport(*report, lines)
		os.Exit(1)
	}

	outFile := filepath.Join(outDir, "Editor.bin")
	st, err := os.Stat(outFile)
	if err != nil || st.Size() == 0 {
		log("Editor.bin missing", map[string]any{"path": outFile, "err": fmt.Sprint(err)})
		writeReport(*report, lines)
		os.Exit(1)
	}
	log("ToEditorBin ok", map[string]any{"editorBinBytes": st.Size()})
	writeReport(*report, lines)
	fmt.Println("\nPASS: x2t conversion works. If demo still fails, the issue is in coauthoring/cache URLs.")
}

func writeReport(path string, lines []string) {
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
