//go:build linux

package convert_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/runlimit"
	"github.com/quantumx-apps/go-office/internal/testutil"
)

func TestMeasureX2TMemoryProfile(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)

	samples := map[string]string{
		"docx": filepath.Join(repo, "sample-files", "sample.docx"),
		"xlsx": filepath.Join(repo, "sample-files", "sample.xlsx"),
		"pptx": filepath.Join(repo, "sample-files", "sample.pptx"),
		"doc":  filepath.Join(repo, "sample-files", "sample.doc"),
	}
	for label, path := range samples {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("sample %s missing: %v", label, err)
		}
	}

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	results, err := convert.BenchmarkX2TMemory(ctx, assets, conv, samples, 6)
	if err != nil {
		t.Fatal(err)
	}

	var maxSinglePeak int64
	var concurrent convert.X2TMemorySample
	var idleBaseline int64
	for _, sample := range results {
		t.Logf("x2t memory %s baseline=%d peak=%d delta=%d",
			sample.Label, sample.BaselineRSSBytes, sample.PeakRSSBytes, sample.PeakDeltaRSSBytes)
		switch {
		case sample.Label == "idle-baseline":
			idleBaseline = sample.BaselineRSSBytes
		case sample.ConcurrentSlots > 0:
			concurrent = sample
		case sample.PeakRSSBytes > maxSinglePeak:
			maxSinglePeak = sample.PeakRSSBytes
		}
	}

	profile := runlimit.X2TMemoryProfile{
		BaselineRSSBytes:       idleBaseline,
		SinglePeakRSSBytes:     maxSinglePeak,
		ConcurrentPeakRSSBytes: concurrent.PeakRSSBytes,
		ConcurrentSlots:        concurrent.ConcurrentSlots,
	}
	if profile.ConcurrentSlots == 0 {
		profile.ConcurrentSlots = 6
	}

	encoded, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("x2t memory profile JSON:\n%s", string(encoded))
	t.Logf("per-slot RSS=%d bytes", profile.PerSlotRSS())

	res := runlimit.ReadResources()
	limits := runlimit.RecommendLimits(res, profile)
	t.Logf("recommended limits workers=%d convert=%d budget=%d cpus=%.2f",
		limits.PlaywrightWorkers, limits.ConvertLimit, limits.MemoryBudgetBytes, limits.CPUs)

	if os.Getenv("X2T_MEMORY_PROFILE_OUT") != "" {
		out := os.Getenv("X2T_MEMORY_PROFILE_OUT")
		if err := os.WriteFile(out, encoded, 0o644); err != nil {
			t.Fatalf("write profile: %v", err)
		}
		t.Logf("wrote profile to %s", out)
	}
}

func TestRecommendLimitsUsesMeasuredProfile(t *testing.T) {
	profile := runlimit.X2TMemoryProfile{
		BaselineRSSBytes:       8 * 1024 * 1024,
		SinglePeakRSSBytes:     28 * 1024 * 1024,
		ConcurrentPeakRSSBytes: 100 * 1024 * 1024,
		ConcurrentSlots:        6,
	}
	res := runlimit.Resources{
		MemoryLimitBytes: 7 * 1024 * 1024 * 1024,
		CPUs:             4,
	}
	limits := runlimit.RecommendLimits(res, profile)
	if limits.ConvertLimit < 1 || limits.ConvertLimit > 4 {
		t.Fatalf("unexpected convert limit: %d", limits.ConvertLimit)
	}
	if limits.PlaywrightWorkers < 1 || limits.PlaywrightWorkers > 10 {
		t.Fatalf("unexpected workers: %d", limits.PlaywrightWorkers)
	}
	t.Logf("example limits: workers=%d convert=%d perSlot=%d",
		limits.PlaywrightWorkers, limits.ConvertLimit, profile.PerSlotRSS())
}
