package assetfetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Options configures Fetch.
type Options struct {
	OutDir      string
	VersionFile string
	Force       bool
	Client      *http.Client
}

// Fetch downloads and extracts Euro-Office assets into OutDir (Linux only).
func Fetch(opts Options) error {
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

	marker := filepath.Join(opts.OutDir, ".extracted")
	if !opts.Force && isUpToDate(marker, v.Release, opts.OutDir) {
		if needsConverterBin(opts.OutDir) {
			fmt.Println("Converter binaries missing — re-extracting assets...")
		} else {
			fmt.Printf("assets already present for %s (%s)\n", v.Release, opts.OutDir)
			if err := ensureConverterExecutables(filepath.Join(opts.OutDir, "converter", "bin")); err != nil {
				return err
			}
			if !FontsReady(opts.OutDir) {
				fmt.Println("Font files incomplete — regenerating AllFonts.js...")
				if err := ensureFontToolchain(opts, v); err != nil {
					return err
				}
			}
			return GenerateAllFonts(opts.OutDir)
		}
	}
	if !opts.Force && needsFontGeneration(opts.OutDir) {
		fmt.Println("Generating missing AllFonts.js for existing assets...")
		if err := ensureFontToolchain(opts, v); err != nil {
			return err
		}
		return GenerateAllFonts(opts.OutDir)
	}

	tmpRoot, err := os.MkdirTemp("", "go-office-fetch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)

	debPath := filepath.Join(tmpRoot, "package.deb")
	url := DebURL(v)
	fmt.Printf("Downloading %s\n", url)
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
	fmt.Printf("Extracting from package (%s)\n", dsRoot)

	if err := os.RemoveAll(opts.OutDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return err
	}

	if err := copyTree(filepath.Join(dsRoot, "web-apps"), filepath.Join(opts.OutDir, "web-apps")); err != nil {
		return err
	}
	if err := copyTree(filepath.Join(dsRoot, "sdkjs"), filepath.Join(opts.OutDir, "sdkjs")); err != nil {
		return err
	}
	if err := copyConverterBin(dsRoot, opts.OutDir); err != nil {
		return err
	}
	fontsSrc := filepath.Join(dsRoot, "fonts")
	if st, err := os.Stat(fontsSrc); err == nil && st.IsDir() {
		if err := copyTree(fontsSrc, filepath.Join(opts.OutDir, "fonts")); err != nil {
			return err
		}
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

	if err := writeMetadata(opts.OutDir, v, url); err != nil {
		return err
	}

	if err := GenerateAllFonts(opts.OutDir); err != nil {
		return err
	}

	fmt.Printf("Done. Set GO_OFFICE_ASSETS=%s and ProtocolVersion=%s\n", opts.OutDir, v.Protocol)
	return nil
}

func needsConverterBin(outDir string) bool {
	x2t := filepath.Join(outDir, "converter", "bin", "x2t")
	st, err := os.Stat(x2t)
	return err != nil || st.IsDir()
}

func isUpToDate(marker, release, outDir string) bool {
	b, err := os.ReadFile(marker)
	if err != nil {
		return false
	}
	if string(b) != release {
		return false
	}
	apiJs := filepath.Join(outDir, "web-apps", "apps", "api", "documents", "api.js")
	apiTpl := filepath.Join(outDir, "web-apps", "apps", "api", "documents", "api.js.tpl")
	if _, err := os.Stat(apiJs); err == nil {
		if FontsReady(outDir) {
			return !needsConverterBin(outDir)
		}
		return false
	}
	if _, err := os.Stat(apiTpl); err == nil {
		return true
	}
	return false
}

func downloadFile(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func writeMetadata(outDir string, v Version, debURL string) error {
	apiDir := filepath.Join(outDir, "web-apps", "apps", "api", "documents")
	apiTpl := filepath.Join(apiDir, "api.js.tpl")
	apiJs := filepath.Join(apiDir, "api.js")
	if st, err := os.Stat(apiTpl); err == nil && !st.IsDir() {
		if _, err := os.Stat(apiJs); os.IsNotExist(err) {
			if err := copyFile(apiTpl, apiJs); err != nil {
				return err
			}
		}
	}

	if err := os.WriteFile(filepath.Join(outDir, "VERSION"), []byte(v.Protocol), 0o644); err != nil {
		return err
	}
	provenance := fmt.Sprintf(`source=Euro-Office/DocumentServer
release=v%s
protocol=%s
fetched=%s
package=%s
license=AGPL-3.0-only
homepage=https://github.com/Euro-Office/DocumentServer
`, v.Release, v.Protocol, time.Now().UTC().Format(time.RFC3339), debURL)
	if err := os.WriteFile(filepath.Join(outDir, "PROVENANCE"), []byte(provenance), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, ".extracted"), []byte(v.Release), 0o644)
}
