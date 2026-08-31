package assetfetch

import (
	"os"
	"path/filepath"
)

func findConverterBin(dsRoot string) string {
	candidates := []string{
		filepath.Join(dsRoot, "server", "FileConverter", "bin"),
		filepath.Join(dsRoot, "FileConverter", "bin"),
	}
	for _, dir := range candidates {
		if st, err := os.Stat(filepath.Join(dir, "x2t")); err == nil && !st.IsDir() {
			return dir
		}
	}
	return ""
}

func copyConverterBin(dsRoot, outDir string) error {
	src := findConverterBin(dsRoot)
	if src == "" {
		return nil
	}
	dst := filepath.Join(outDir, "converter", "bin")
	if err := copyTree(src, dst); err != nil {
		return err
	}
	if err := fixDoctRendererConfig(dst); err != nil {
		return err
	}
	return ensureConverterExecutables(dst)
}

func ensureConverterExecutables(binDir string) error {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(binDir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode&0o111 != 0 {
			continue
		}
		if err := os.Chmod(path, mode.Perm()|0o755); err != nil {
			return err
		}
	}
	return nil
}
