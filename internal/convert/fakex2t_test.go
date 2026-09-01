package convert

import "testing"

func TestFakeX2TRejectsSharedAllFontsPath(t *testing.T) {
	task := FakeX2TTask{
		IsolatedDir:  "/run/x2t-abc",
		AllFontsPath: "/assets/converter/bin/AllFonts.js",
		FromChanges:  true,
		FontDir:      "/run/x2t-abc",
	}
	if err := validateFakeX2TTask(task); err == nil {
		t.Fatal("expected shared all fonts path to be rejected")
	}
}
