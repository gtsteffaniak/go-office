package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FakeX2TTask records one injected x2t invocation.
type FakeX2TTask struct {
	TaskPath     string
	IsolatedDir  string
	XML          string
	FileFrom     string
	FileTo       string
	AllFontsPath string
	FontDir      string
	FromChanges  bool
}

// FakeX2T stubs x2t: validates font isolation in task XML and writes minimal output.
type FakeX2T struct {
	Tasks []FakeX2TTask
}

// Run implements X2TRunner.
func (f *FakeX2T) Run(_ context.Context, taskPath, isolatedDir string) ([]byte, error) {
	raw, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, err
	}
	xml := string(raw)
	task := FakeX2TTask{
		TaskPath:     taskPath,
		IsolatedDir:  isolatedDir,
		XML:          xml,
		FileFrom:     x2tTaskTag(xml, "m_sFileFrom"),
		FileTo:       x2tTaskTag(xml, "m_sFileTo"),
		AllFontsPath: x2tTaskTag(xml, "m_sAllFontsPath"),
		FontDir:      x2tTaskTag(xml, "m_sFontDir"),
		FromChanges:  strings.Contains(xml, "<m_bFromChanges>true</m_bFromChanges>"),
	}
	if err := validateFakeX2TTask(task); err != nil {
		return nil, err
	}
	if task.FileTo == "" {
		return nil, fmt.Errorf("fake x2t: m_sFileTo missing in %s", taskPath)
	}
	if err := os.MkdirAll(filepath.Dir(task.FileTo), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(task.FileTo, []byte("fake-x2t-output"), 0o644); err != nil {
		return nil, err
	}
	f.Tasks = append(f.Tasks, task)
	return nil, nil
}

func validateFakeX2TTask(task FakeX2TTask) error {
	if task.AllFontsPath == "" {
		return nil
	}
	if task.IsolatedDir == "" {
		return fmt.Errorf("fake x2t: all fonts path set but isolated dir empty")
	}
	allFonts := filepath.Clean(task.AllFontsPath)
	isolated := filepath.Clean(task.IsolatedDir)
	if allFonts != isolated && !strings.HasPrefix(allFonts, isolated+string(filepath.Separator)) {
		return fmt.Errorf("fake x2t: all fonts path %q must be under isolated dir %q", allFonts, isolated)
	}
	if task.FromChanges && task.FontDir != "" {
		fontDir := filepath.Clean(task.FontDir)
		if fontDir != isolated && !strings.HasPrefix(fontDir, isolated+string(filepath.Separator)) {
			return fmt.Errorf("fake x2t: font dir %q must be under isolated dir %q for fromChanges", fontDir, isolated)
		}
	}
	return nil
}

func x2tTaskTag(xml, tag string) string {
	start := "<" + tag + ">"
	end := "</" + tag + ">"
	i := strings.Index(xml, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := strings.Index(xml[i:], end)
	if j < 0 {
		return ""
	}
	return xml[i : i+j]
}
