package assetfetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimalSpellcheckJS = `spellcheckGetLanguages=function(){return{"1033":{"hyphen":true,"name":"en_US"},"1031":{"hyphen":true,"name":"de_DE"}}}`

func TestSpellcheckLanguageNames(t *testing.T) {
	dir := t.TempDir()
	sdk := filepath.Join(dir, "sdkjs", "word")
	if err := os.MkdirAll(sdk, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := "/* test */" + minimalSpellcheckJS + "/* end */"
	if err := os.WriteFile(filepath.Join(sdk, "sdk-all-min.js"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	langs, err := SpellcheckLanguageNames(filepath.Join(dir, "sdkjs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(langs) != 2 || langs[0] != "de_DE" || langs[1] != "en_US" {
		t.Fatalf("langs = %v", langs)
	}
}

func TestInstallDictionariesFiltersFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	for _, lang := range []string{"en_US", "de_DE"} {
		langDir := filepath.Join(src, lang)
		if err := os.MkdirAll(langDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{
			lang + ".aff",
			lang + ".dic",
			"hyph_" + lang + ".dic",
			"license.txt",
			lang + "_thes.dat",
			lang + ".json",
		} {
			if err := os.WriteFile(filepath.Join(langDir, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	sdk := filepath.Join(dst, "sdkjs", "word")
	if err := os.MkdirAll(sdk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdk, "sdk-all-min.js"), []byte(minimalSpellcheckJS), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallDictionaries(src, dst); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en_US", "de_DE"} {
		langDir := filepath.Join(dst, "dictionaries", lang)
		for _, want := range []string{lang + ".aff", lang + ".dic", "license.txt"} {
			if _, err := os.Stat(filepath.Join(langDir, want)); err != nil {
				t.Fatalf("%s: missing %s: %v", lang, want, err)
			}
		}
		if _, err := os.Stat(filepath.Join(langDir, lang+"_thes.dat")); err == nil {
			t.Fatalf("%s: thesaurus file should not be copied", lang)
		}
	}
}

func TestShouldCopyDictionaryFile(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"en_US.aff", true},
		{"en_US.dic", true},
		{"hyph_en_US.dic", true},
		{"license.txt", true},
		{"COPYING_GPLv3", true},
		{"README_en_US.txt", true},
		{"en_US_thes.dat", false},
		{"en_US.json", false},
	}
	for _, tc := range cases {
		if got := shouldCopyDictionaryFile(tc.name, "en_US"); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestDictionariesReadyRequiresSpellEngine(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSpellEngine(t, dir)
	writeMinimalDictionaryPair(t, dir, "en_US")
	writeMinimalDictionaryPair(t, dir, "de_DE")
	if err := os.WriteFile(filepath.Join(dir, "dictionaries", ".installed-langs"), []byte("en_US\nde_DE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !DictionariesReady(dir) {
		t.Fatal("expected ready")
	}
	_ = os.Remove(filepath.Join(dir, "sdkjs", "common", "spell", "spell", "spell.wasm"))
	if DictionariesReady(dir) {
		t.Fatal("expected missing wasm to make tree incomplete")
	}
}

func writeMinimalSpellEngine(t *testing.T, dir string) {
	t.Helper()
	spellDir := filepath.Join(dir, "sdkjs", "common", "spell", "spell")
	if err := os.MkdirAll(spellDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"spell.js", "spell.wasm"} {
		if err := os.WriteFile(filepath.Join(spellDir, name), []byte("spell"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeMinimalDictionaryPair(t *testing.T, dir, lang string) {
	t.Helper()
	langDir := filepath.Join(dir, "dictionaries", lang)
	if err := os.MkdirAll(langDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".aff", ".dic"} {
		if err := os.WriteFile(filepath.Join(langDir, lang+ext), []byte("dict"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExtractSpellcheckObject(t *testing.T) {
	src := "prefix" + minimalSpellcheckJS + "suffix"
	chunk, ok := extractSpellcheckObject(src)
	if !ok || !strings.Contains(chunk, "en_US") {
		t.Fatalf("chunk = %q ok=%v", chunk, ok)
	}
}
