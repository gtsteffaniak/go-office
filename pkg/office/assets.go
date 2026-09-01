package office

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantumx-apps/go-office/internal/assetfetch"
)

// AssetOptions configures asset discovery and fetch.
type AssetOptions struct {
	Dir        string // preferred dir; empty → resolve via env + defaults
	Logger     *slog.Logger
	HTTPClient *http.Client // optional; for tests
}

// AssetBundle describes a resolved Euro-Office asset tree.
type AssetBundle struct {
	Dir     string
	Version string // Euro-Office release; same value as assets/VERSION and ProtocolVersion
}

// EnsureAssetsOptions configures EnsureAssets.
type EnsureAssetsOptions struct {
	AssetOptions
	Force bool // re-fetch even if valid assets already exist at Dir
}

// ValidAssetDir reports whether dir contains a complete asset tree (any Euro-Office version).
func ValidAssetDir(dir string) bool {
	return assetfetch.ValidAssetDir(dir)
}

// DiscoverAssets scans candidate directories for a complete asset tree.
// No network I/O. Returns (bundle, true, nil) when found, (zero, false, nil) when not.
func DiscoverAssets(opts AssetOptions) (AssetBundle, bool, error) {
	for _, dir := range assetCandidateDirs(opts.Dir) {
		if !assetfetch.ValidAssetDir(dir) {
			continue
		}
		bundle := AssetBundle{
			Dir:     dir,
			Version: assetfetch.ReadInstalledVersion(dir),
		}
		warnAssetVersionMismatch(opts.Logger, bundle.Version)
		return bundle, true, nil
	}
	return AssetBundle{}, false, nil
}

// FetchAssets downloads the library-pinned Euro-Office release into Dir.
// Caller must invoke explicitly when they want a download to happen.
func FetchAssets(ctx context.Context, opts AssetOptions) (AssetBundle, error) {
	return fetchAssets(ctx, opts, false)
}

func fetchAssets(ctx context.Context, opts AssetOptions, force bool) (AssetBundle, error) {
	dir := fetchTargetDir(opts.Dir)
	if dir == "" {
		return AssetBundle{}, fmt.Errorf("office: asset directory is required for fetch")
	}
	if err := assetfetch.FetchContext(ctx, assetfetch.Options{
		OutDir:  dir,
		Version: ExpectedAssetsVersion,
		Force:   force,
		Client:  opts.HTTPClient,
	}); err != nil {
		return AssetBundle{}, err
	}
	version := assetfetch.ReadInstalledVersion(dir)
	if version == "" {
		version = ExpectedAssetsVersion
	}
	return AssetBundle{Dir: dir, Version: version}, nil
}

// EnsureAssets discovers existing assets or fetches the pinned release when missing.
// Equivalent to DiscoverAssets followed by FetchAssets; still requires an explicit call.
func EnsureAssets(ctx context.Context, opts EnsureAssetsOptions) (AssetBundle, error) {
	if !opts.Force {
		if bundle, ok, err := DiscoverAssets(opts.AssetOptions); err != nil {
			return AssetBundle{}, err
		} else if ok {
			return bundle, nil
		}
	}
	return fetchAssets(ctx, opts.AssetOptions, opts.Force)
}

func assetCandidateDirs(preferred string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(dir string) {
		if dir == "" {
			return
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}

	add(preferred)
	add(AssetDirFromEnv(""))
	if wd, err := os.Getwd(); err == nil {
		add(filepath.Join(wd, "assets"))
	}
	add(defaultAssetCacheDir())

	return out
}

func fetchTargetDir(preferred string) string {
	if preferred != "" {
		return preferred
	}
	if v := AssetDirFromEnv(""); v != "" {
		return v
	}
	return defaultAssetCacheDir()
}

func defaultAssetCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "go-office", "assets")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "go-office", "assets")
}

func warnAssetVersionMismatch(logger *slog.Logger, installed string) {
	if installed == "" || installed == ExpectedAssetsVersion {
		return
	}
	msg := fmt.Sprintf(
		"office: using Euro-Office assets %s (this build expects %s)",
		installed, ExpectedAssetsVersion,
	)
	if logger != nil {
		logger.Warn(msg)
	} else {
		slog.Default().Warn(msg)
	}
}

// AssetDirFromEnv returns OFFICE_ASSETS, or fallback when unset.
func AssetDirFromEnv(fallback string) string {
	if v := strings.TrimSpace(os.Getenv("OFFICE_ASSETS")); v != "" {
		return v
	}
	return fallback
}

// ReadAssetVersion reads assets/VERSION written by fetch-assets.
func ReadAssetVersion(assetDir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(assetDir, "VERSION"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
