package assetfetch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateAllFonts builds sdkjs/common/AllFonts.js using the Euro-Office allfontsgen tool.
func GenerateAllFonts(outDir string) error {
	if err := RequireLinux(); err != nil {
		return err
	}

	allFontsWeb := filepath.Join(outDir, "sdkjs", "common", "AllFonts.js")
	if st, err := os.Stat(allFontsWeb); err == nil && st.Size() > 0 {
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

	fmt.Println("Generating AllFonts.js (may take a minute)...")
	cmd := exec.Command(gen,
		"--input="+coreFonts,
		"--allfonts-web="+allFontsWeb,
		"--allfonts="+filepath.Join(converterBin, "AllFonts.js"),
		"--images="+images,
		"--selection="+filepath.Join(converterBin, "font_selection.bin"),
		"--output-web="+fontsOut,
		"--use-system=true",
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
	fmt.Println("AllFonts.js ready")
	return nil
}

func needsFontGeneration(outDir string) bool {
	apiJs := filepath.Join(outDir, "web-apps", "apps", "api", "documents", "api.js")
	apiTpl := filepath.Join(outDir, "web-apps", "apps", "api", "documents", "api.js.tpl")
	if _, err := os.Stat(apiJs); err != nil {
		if _, err := os.Stat(apiTpl); err != nil {
			return false
		}
	}
	allFonts := filepath.Join(outDir, "sdkjs", "common", "AllFonts.js")
	if st, err := os.Stat(allFonts); err == nil && st.Size() > 0 {
		return false
	}
	return true
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
	convSrc := filepath.Join(dsRoot, "server", "FileConverter", "bin")
	if st, err := os.Stat(convSrc); err == nil && st.IsDir() {
		if err := copyTree(convSrc, filepath.Join(opts.OutDir, "converter", "bin")); err != nil {
			return err
		}
	}
	return nil
}
