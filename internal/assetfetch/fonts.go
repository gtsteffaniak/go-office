package assetfetch

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// GenerateAllFonts builds sdkjs/common/AllFonts.js using the Euro-Office allfontsgen tool.
// Output paths are absolute for the current OutDir/host. Converter.New remaps them at
// runtime so Docker copies (e.g. /app/assets) still resolve. --use-system=false keeps
// faces inside core-fonts instead of /usr/share/fonts.
func GenerateAllFonts(outDir string) error {
	if err := RequireLinux(); err != nil {
		return err
	}

	if FontsReady(outDir) {
		return nil
	}

	gen := filepath.Join(outDir, "tools", "allfontsgen")
	if st, err := os.Stat(gen); err != nil || st.IsDir() {
		return fmt.Errorf("assetfetch: allfontsgen not found at %s (re-run fetch-assets)", gen)
	}

	converterBin := filepath.Join(outDir, "converter", "bin")
	coreFonts := filepath.Join(outDir, "core-fonts")
	images := filepath.Join(outDir, "sdkjs", "common", "Images")
	fontsOut := filepath.Join(outDir, "fonts")

	for _, p := range []string{converterBin, coreFonts, images} {
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return fmt.Errorf("assetfetch: missing %s (re-run fetch-assets)", p)
		}
	}
	if err := os.MkdirAll(fontsOut, 0o755); err != nil {
		return err
	}

	allFontsWeb := filepath.Join(outDir, "sdkjs", "common", "AllFonts.js")
	fmt.Println("Generating AllFonts.js (may take a minute)...")
	cmd := exec.Command(gen,
		"--input="+coreFonts,
		"--allfonts-web="+allFontsWeb,
		"--allfonts="+filepath.Join(converterBin, "AllFonts.js"),
		"--images="+images,
		"--selection="+filepath.Join(converterBin, "font_selection.bin"),
		"--output-web="+fontsOut,
		"--use-system=false",
		"--use-system-user-fonts=false",
	)
	cmd.Dir = outDir
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+converterBin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("assetfetch: allfontsgen: %w", err)
	}

	if st, err := os.Stat(allFontsWeb); err != nil || st.Size() == 0 {
		return fmt.Errorf("assetfetch: AllFonts.js was not created")
	}
	fmt.Println("AllFonts.js ready (absolute paths are host-specific; Converter remaps them at runtime)")
	return nil
}

const minWebAllFontsBytes = 64 * 1024

// FontsReady reports whether editor font bundles were generated.
func FontsReady(outDir string) bool {
	checks := []struct {
		path string
		min  int64
	}{
		{filepath.Join(outDir, "sdkjs", "common", "AllFonts.js"), minWebAllFontsBytes},
		{filepath.Join(outDir, "converter", "bin", "font_selection.bin"), 1024},
		{filepath.Join(outDir, "converter", "bin", "AllFonts.js"), 1024},
	}
	for _, c := range checks {
		st, err := os.Stat(c.path)
		if err != nil || st.Size() < c.min {
			return false
		}
	}
	return true
}

// RegenerateFonts rebuilds font cache files, optionally deleting stale outputs first.
func RegenerateFonts(opts Options, force bool) error {
	if err := RequireLinux(); err != nil {
		return err
	}
	if opts.OutDir == "" {
		return fmt.Errorf("assetfetch: OutDir is required")
	}
	if opts.VersionFile == "" {
		return fmt.Errorf("assetfetch: VersionFile is required")
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Minute}
	}
	v, err := LoadVersion(opts.VersionFile)
	if err != nil {
		return err
	}
	if force {
		for _, path := range []string{
			filepath.Join(opts.OutDir, "sdkjs", "common", "AllFonts.js"),
			filepath.Join(opts.OutDir, "converter", "bin", "font_selection.bin"),
			filepath.Join(opts.OutDir, "converter", "bin", "AllFonts.js"),
		} {
			_ = os.Remove(path)
		}
	}
	if !FontsReady(opts.OutDir) {
		if err := ensureFontToolchain(opts, v); err != nil {
			return err
		}
	}
	return GenerateAllFonts(opts.OutDir)
}

func needsFontGeneration(outDir string) bool {
	return !FontsReady(outDir)
}

// FontGenerationPossible reports whether the on-disk tree has enough to run allfontsgen
// without a full package re-extract (needs sdkjs/common/Images, core-fonts, converter, tool).
func FontGenerationPossible(outDir string) bool {
	return fontGenerationPossible(outDir)
}

func fontGenerationPossible(outDir string) bool {
	if needsConverterBin(outDir) {
		return false
	}
	for _, p := range []string{
		filepath.Join(outDir, "converter", "bin"),
		filepath.Join(outDir, "core-fonts"),
		filepath.Join(outDir, "sdkjs", "common", "Images"),
	} {
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			return false
		}
	}
	gen := filepath.Join(outDir, "tools", "allfontsgen")
	st, err := os.Stat(gen)
	return err == nil && !st.IsDir()
}

func ensureFontToolchain(opts Options, v Version) error {
	tmpRoot, err := os.MkdirTemp("", "go-office-fonts-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)

	debPath := filepath.Join(tmpRoot, "package.deb")
	url := DebURL(v)
	fmt.Printf("Downloading toolchain %s\n", url)
	if err := downloadFile(opts.Client, url, debPath); err != nil {
		return err
	}

	extractRoot := filepath.Join(tmpRoot, "root")
	if err := ExtractDebData(debPath, extractRoot); err != nil {
		return err
	}
	dsRoot, err := FindDocumentServerRoot(extractRoot)
	if err != nil {
		return err
	}

	coreFontsSrc := filepath.Join(dsRoot, "core-fonts")
	if st, err := os.Stat(coreFontsSrc); err == nil && st.IsDir() {
		if err := copyTree(coreFontsSrc, filepath.Join(opts.OutDir, "core-fonts")); err != nil {
			return err
		}
	}
	toolsSrc := filepath.Join(dsRoot, "server", "tools", "allfontsgen")
	if st, err := os.Stat(toolsSrc); err == nil && !st.IsDir() {
		toolsDst := filepath.Join(opts.OutDir, "tools", "allfontsgen")
		if err := copyFile(toolsSrc, toolsDst); err != nil {
			return err
		}
		if err := os.Chmod(toolsDst, 0o755); err != nil {
			return err
		}
	}
	return copyConverterBin(dsRoot, opts.OutDir)
}
