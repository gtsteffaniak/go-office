package assetfetch

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// dictionariesTarballSHA256 is the sha256 of the Euro-Office/dictionaries release archive
// used when the Document Server .deb does not ship dictionaries/.
const dictionariesTarballSHA256 = "193590a8ed9da45b58c869dbb147ec8daa920ff3a17998bb103e1c50866f9ba3"

var spellcheckLangRe = regexp.MustCompile(`"name":"([^"]+)"`)

// SpellEngineReady reports whether browser Hunspell engine files are present under sdkjs.
func SpellEngineReady(outDir string) bool {
	for _, name := range []string{"spell.js", "spell.wasm"} {
		path := filepath.Join(outDir, "sdkjs", "common", "spell", "spell", name)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() || st.Size() == 0 {
			return false
		}
	}
	return true
}

const installedDictionaryLangsFile = ".installed-langs"

// DictionariesReady reports whether spellcheck dictionaries for required locales exist.
func DictionariesReady(outDir string) bool {
	if !SpellEngineReady(outDir) {
		return false
	}
	langs, err := readInstalledDictionaryLangs(outDir)
	if err != nil || len(langs) == 0 {
		return false
	}
	for _, lang := range langs {
		if !dictionaryPairReady(outDir, lang) {
			return false
		}
	}
	return true
}

func readInstalledDictionaryLangs(outDir string) ([]string, error) {
	manifest := filepath.Join(outDir, "dictionaries", installedDictionaryLangsFile)
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}
	var langs []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			langs = append(langs, line)
		}
	}
	return langs, nil
}

func writeInstalledDictionaryLangs(outDir string, langs []string) error {
	var b strings.Builder
	for _, lang := range langs {
		b.WriteString(lang)
		b.WriteByte('\n')
	}
	path := filepath.Join(outDir, "dictionaries", installedDictionaryLangsFile)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func dictionaryPairReady(outDir, lang string) bool {
	for _, ext := range []string{".aff", ".dic"} {
		path := filepath.Join(outDir, "dictionaries", lang, lang+ext)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() || st.Size() == 0 {
			return false
		}
	}
	return true
}

// SpellcheckLanguageNames returns unique Hunspell folder ids from sdkjs spellcheckGetLanguages().
func SpellcheckLanguageNames(sdkjsDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(sdkjsDir, "word", "sdk-all-min.js"))
	if err != nil {
		return nil, err
	}
	chunk, ok := extractSpellcheckObject(string(data))
	if !ok {
		return nil, fmt.Errorf("assetfetch: spellcheckGetLanguages not found in sdkjs")
	}
	names := make(map[string]struct{})
	for _, m := range spellcheckLangRe.FindAllStringSubmatch(chunk, -1) {
		if m[1] != "" {
			names[m[1]] = struct{}{}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("assetfetch: no spellcheck languages in sdkjs")
	}
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

func extractSpellcheckObject(src string) (string, bool) {
	const marker = "spellcheckGetLanguages=function(){return"
	start := strings.Index(src, marker)
	if start < 0 {
		return "", false
	}
	i := start + len(marker)
	if i >= len(src) || src[i] != '{' {
		return "", false
	}
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1], true
			}
		}
	}
	return "", false
}

func installDictionariesFromPackage(dsRoot, outDir string) error {
	dictSrc := filepath.Join(dsRoot, "dictionaries")
	if st, err := os.Stat(dictSrc); err != nil || !st.IsDir() {
		fmt.Println("Euro-Office package has no dictionaries/ — will fetch from GitHub after sdkjs is installed")
		return nil
	}
	fmt.Println("Installing spellcheck dictionaries from Euro-Office package...")
	return InstallDictionaries(dsRoot, outDir)
}

// InstallDictionaries copies spellcheck dictionaries into outDir/dictionaries.
// srcRoot is the Document Server root (with dictionaries/) or a dictionaries-only tree.
func InstallDictionaries(srcRoot, outDir string) error {
	sdkjsDir := filepath.Join(outDir, "sdkjs")
	langs, err := SpellcheckLanguageNames(sdkjsDir)
	if err != nil {
		return err
	}
	srcDict := filepath.Join(srcRoot, "dictionaries")
	if st, err := os.Stat(srcDict); err != nil || !st.IsDir() {
		if isDictionaryLangDir(srcRoot, langs[0]) {
			srcDict = srcRoot
		} else {
			return fmt.Errorf("assetfetch: dictionaries source not found under %s", srcRoot)
		}
	}
	dstDict := filepath.Join(outDir, "dictionaries")
	if err := os.MkdirAll(dstDict, 0o755); err != nil {
		return err
	}
	for _, lang := range langs {
		if err := copyDictionaryLang(srcDict, dstDict, lang); err != nil {
			return err
		}
	}
	return writeInstalledDictionaryLangs(outDir, langs)
}

func isDictionaryLangDir(root, lang string) bool {
	aff := filepath.Join(root, lang, lang+".aff")
	dic := filepath.Join(root, lang, lang+".dic")
	stAff, errAff := os.Stat(aff)
	stDic, errDic := os.Stat(dic)
	return errAff == nil && errDic == nil && !stAff.IsDir() && !stDic.IsDir()
}

func copyDictionaryLang(srcDict, dstDict, lang string) error {
	srcLang := filepath.Join(srcDict, lang)
	st, err := os.Stat(srcLang)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("assetfetch: dictionary %q missing under %s", lang, srcDict)
	}
	dstLang := filepath.Join(dstDict, lang)
	if err = os.MkdirAll(dstLang, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcLang)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !shouldCopyDictionaryFile(name, lang) {
			continue
		}
		if err := copyFile(filepath.Join(srcLang, name), filepath.Join(dstLang, name)); err != nil {
			return err
		}
	}
	return nil
}

func shouldCopyDictionaryFile(name, lang string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".aff"), strings.HasSuffix(lower, ".dic"):
		return true
	case strings.HasPrefix(lower, "hyph_") && strings.HasSuffix(lower, ".dic"):
		return true
	case strings.Contains(lower, "license"), strings.Contains(lower, "copying"),
		strings.Contains(lower, "authors"), strings.HasPrefix(lower, "readme"),
		strings.Contains(lower, "wordnet"):
		return true
	case name == lang+".json":
		return false
	default:
		return false
	}
}

// EnsureDictionaries installs dictionaries when missing, using the pinned .deb or GitHub fallback.
func EnsureDictionaries(ctx context.Context, opts Options, version, outDir string) error {
	if DictionariesReady(outDir) {
		return nil
	}
	if err := RequireLinux(); err != nil {
		return err
	}
	if opts.Client == nil {
		opts.Client = defaultHTTPClient()
	}

	tmpRoot, err := os.MkdirTemp("", "go-office-dict-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)

	debPath := filepath.Join(tmpRoot, "package.deb")
	url := DebURL(version)
	fmt.Printf("Downloading dictionaries toolchain %s\n", url)
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
	dictSrc := filepath.Join(dsRoot, "dictionaries")
	if st, err := os.Stat(dictSrc); err == nil && st.IsDir() {
		fmt.Println("Installing spellcheck dictionaries from Euro-Office package...")
		return InstallDictionaries(dsRoot, outDir)
	}
	fmt.Println("Package has no dictionaries/ — fetching Euro-Office/dictionaries release...")
	return fetchDictionariesArchive(ctx, opts.Client, version, outDir)
}

func fetchDictionariesArchive(ctx context.Context, client *http.Client, version, outDir string) error {
	tmpRoot, err := os.MkdirTemp("", "go-office-dict-tar-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)

	archivePath := filepath.Join(tmpRoot, "dictionaries.tar.gz")
	url := fmt.Sprintf("https://codeload.github.com/Euro-Office/dictionaries/tar.gz/refs/tags/v%s", version)
	if err = downloadFile(ctx, client, url, archivePath); err != nil {
		return err
	}
	sum, err := fileSHA256(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, dictionariesTarballSHA256) {
		return fmt.Errorf("assetfetch: dictionaries archive checksum mismatch (got %s)", sum)
	}
	extractDir := filepath.Join(tmpRoot, "extract")
	if err = os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err = extractTarGz(archivePath, extractDir); err != nil {
		return err
	}
	srcRoot, err := findDictionaryArchiveRoot(extractDir)
	if err != nil {
		return err
	}
	return InstallDictionaries(srcRoot, outDir)
}

func findDictionaryArchiveRoot(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		candidate := filepath.Join(dir, ent.Name())
		sub, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}
		for _, s := range sub {
			if s.IsDir() && strings.Contains(s.Name(), "_") {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("assetfetch: dictionary archive layout not recognized under %s", dir)
}

func extractTarGz(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("assetfetch: tar path escapes dest: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode).Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Minute}
}
