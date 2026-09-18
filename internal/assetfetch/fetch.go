package assetfetch

import (
	"context"
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
	Version     string // pinned release; if empty, loaded from VersionFile
	VersionFile string
	Force       bool
	Client      *http.Client
}

// Fetch downloads and extracts Euro-Office assets into OutDir (Linux only).
func Fetch(opts Options) error {
	return FetchContext(context.Background(), opts)
}

// FetchContext downloads and extracts Euro-Office assets, honouring ctx cancellation.
func FetchContext(ctx context.Context, opts Options) error {
	if err := RequireLinux(); err != nil {
		return err
	}
	if opts.OutDir == "" {
		return fmt.Errorf("assetfetch: OutDir is required")
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Minute}
	}

	version := opts.Version
	if version == "" {
		if opts.VersionFile == "" {
			return fmt.Errorf("assetfetch: Version or VersionFile is required")
		}
		var err error
		version, err = LoadVersion(opts.VersionFile)
		if err != nil {
			return err
		}
	}

	return withFetchLock(opts.OutDir, func() error {
		return fetchLocked(ctx, opts, version)
	})
}

func fetchLocked(ctx context.Context, opts Options, version string) error {
	marker := filepath.Join(opts.OutDir, ".extracted")
	if !opts.Force && isUpToDate(marker, version, opts.OutDir) {
		fmt.Printf("assets already present for %s (%s)\n", version, opts.OutDir)
		if err := ensureConverterExecutables(filepath.Join(opts.OutDir, "converter", "bin")); err != nil {
			return err
		}
		if err := patchSDKJS(opts.OutDir); err != nil {
			return err
		}
		if err := ensureSlideThemesJS(opts.OutDir); err != nil {
			return err
		}
		return fixDoctRendererConfig(filepath.Join(opts.OutDir, "converter", "bin"))
	}
	if needsConverterBin(opts.OutDir) {
		fmt.Println("Converter binaries missing — re-extracting assets...")
	}
	if !opts.Force && needsFontGeneration(opts.OutDir) {
		if fontGenerationPossible(opts.OutDir) {
			fmt.Println("Generating missing AllFonts.js for existing assets...")
			return GenerateAllFonts(opts.OutDir)
		}
		fmt.Println("Asset tree incomplete — re-extracting from package...")
	}

	tmpRoot, err := os.MkdirTemp("", "go-office-fetch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)

	debPath := filepath.Join(tmpRoot, "package.deb")
	url := DebURL(version)
	fmt.Printf("Downloading %s\n", url)
	if err = downloadFile(ctx, opts.Client, url, debPath); err != nil {
		return err
	}

	extractRoot := filepath.Join(tmpRoot, "root")
	if err = ExtractDebData(debPath, extractRoot); err != nil {
		return err
	}

	dsRoot, err := FindDocumentServerRoot(extractRoot)
	if err != nil {
		return err
	}
	fmt.Printf("Extracting from package (%s)\n", dsRoot)

	if err := clearDirContents(opts.OutDir); err != nil {
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

	if err := writeMetadata(opts.OutDir, version, url); err != nil {
		return err
	}

	if err := GenerateAllFonts(opts.OutDir); err != nil {
		return err
	}
	if err := patchSDKJS(opts.OutDir); err != nil {
		return err
	}
	if err := ensureSlideThemesJS(opts.OutDir); err != nil {
		return err
	}

	fmt.Printf("Done. Set OFFICE_ASSETS=%s and ProtocolVersion=%s\n", opts.OutDir, version)
	return nil
}

func needsConverterBin(outDir string) bool {
	x2t := filepath.Join(outDir, "converter", "bin", "x2t")
	st, err := os.Stat(x2t)
	return err != nil || st.IsDir()
}

// IsUpToDate reports whether extracted assets are complete for release.
func IsUpToDate(marker, release, outDir string) bool {
	return isUpToDate(marker, release, outDir)
}

func isUpToDate(marker, release, outDir string) bool {
	b, err := os.ReadFile(marker)
	if err != nil {
		return false
	}
	if string(b) != release {
		return false
	}
	return ValidAssetDir(outDir)
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
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

func writeMetadata(outDir, version, debURL string) error {
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

	if err := os.WriteFile(filepath.Join(outDir, "VERSION"), []byte(version), 0o644); err != nil {
		return err
	}
	provenance := fmt.Sprintf(`source=Euro-Office/DocumentServer
release=v%s
version=%s
fetched=%s
package=%s
license=AGPL-3.0-only
homepage=https://github.com/Euro-Office/DocumentServer
`, version, version, time.Now().UTC().Format(time.RFC3339), debURL)
	if err := os.WriteFile(filepath.Join(outDir, "PROVENANCE"), []byte(provenance), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, ".extracted"), []byte(version), 0o644)
}
