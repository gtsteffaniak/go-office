//go:build linux

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/runlimit"
)

func main() {
	assets := flag.String("assets", "assets", "Euro-Office assets directory")
	repo := flag.String("repo", ".", "repository root containing sample-files/")
	limitsOnly := flag.Bool("limits-only", false, "print PLAYWRIGHT_WORKERS and OFFICE_CONVERT_LIMIT from profile")
	profilePath := flag.String("profile", "", "optional path to write measured profile JSON")
	flag.Parse()

	if *limitsOnly {
		res := runlimit.ReadResources()
		limits := runlimit.RecommendLimits(res, runlimit.DefaultX2TMemoryProfile)
		fmt.Printf("PLAYWRIGHT_WORKERS=%d\n", limits.PlaywrightWorkers)
		fmt.Printf("OFFICE_CONVERT_LIMIT=%d\n", limits.ConvertLimit)
		fmt.Printf("# memory_budget_bytes=%d per_slot_rss_bytes=%d cpus=%.2f\n",
			limits.MemoryBudgetBytes, limits.PerSlotRSSBytes, limits.CPUs)
		return
	}

	samples := map[string]string{
		"docx": filepath.Join(*repo, "sample-files", "sample.docx"),
		"xlsx": filepath.Join(*repo, "sample-files", "sample.xlsx"),
		"pptx": filepath.Join(*repo, "sample-files", "sample.pptx"),
		"doc":  filepath.Join(*repo, "sample-files", "sample.doc"),
	}
	for label, path := range samples {
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintf(os.Stderr, "sample %s missing at %s: %v\n", label, path, err)
			os.Exit(1)
		}
	}

	conv, err := convert.New(convert.Options{AssetDir: *assets, Limit: 1})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	results, err := convert.BenchmarkX2TMemory(ctx, *assets, conv, samples, 6)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var maxSinglePeak int64
	var concurrent convert.X2TMemorySample
	var idleBaseline int64
	for _, sample := range results {
		fmt.Printf("%s baseline=%d peak=%d delta=%d slots=%d\n",
			sample.Label, sample.BaselineRSSBytes, sample.PeakRSSBytes, sample.PeakDeltaRSSBytes, sample.ConcurrentSlots)
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("profile=%s\n", string(encoded))
	fmt.Printf("per_slot_rss_bytes=%d\n", profile.PerSlotRSS())

	res := runlimit.ReadResources()
	limits := runlimit.RecommendLimits(res, profile)
	fmt.Printf("PLAYWRIGHT_WORKERS=%d\n", limits.PlaywrightWorkers)
	fmt.Printf("OFFICE_CONVERT_LIMIT=%d\n", limits.ConvertLimit)

	if *profilePath != "" {
		if err := os.WriteFile(*profilePath, encoded, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write profile: %v\n", err)
			os.Exit(1)
		}
	}
}
