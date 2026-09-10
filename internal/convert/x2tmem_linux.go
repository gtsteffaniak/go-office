//go:build linux

package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// X2TMemorySample captures peak x2t RSS observed during a workload.
type X2TMemorySample struct {
	Label              string
	BaselineRSSBytes   int64
	PeakRSSBytes       int64
	PeakDeltaRSSBytes  int64
	ConcurrentSlots    int
}

// SumX2TRSSBytes returns the combined VmRSS of running x2t processes.
func SumX2TRSSBytes() int64 {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	var total int64
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid <= 0 {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", ent.Name(), "comm"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(comm)) != "x2t" {
			continue
		}
		total += processRSSBytes(pid)
	}
	return total
}

func processRSSBytes(pid int) int64 {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					return kb * 1024
				}
			}
		}
	}
	return 0
}

// MeasurePeakX2TRSS runs fn while polling x2t RSS and returns the peak total.
func MeasurePeakX2TRSS(ctx context.Context, pollInterval time.Duration, fn func() error) (baseline, peak int64, err error) {
	if pollInterval <= 0 {
		pollInterval = 25 * time.Millisecond
	}
	baseline = SumX2TRSSBytes()
	peak = baseline

	done := make(chan struct{})
	var runErr error
	go func() {
		runErr = fn()
		close(done)
	}()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			if rss := SumX2TRSSBytes(); rss > peak {
				peak = rss
			}
			return baseline, peak, runErr
		case <-ctx.Done():
			return baseline, peak, ctx.Err()
		case <-ticker.C:
			if rss := SumX2TRSSBytes(); rss > peak {
				peak = rss
			}
		}
	}
}

// BenchmarkX2TMemory runs representative conversions and returns peak RSS samples.
func BenchmarkX2TMemory(ctx context.Context, assetDir string, conv *Converter, samples map[string]string, concurrentSlots int) ([]X2TMemorySample, error) {
	if conv == nil {
		return nil, fmt.Errorf("convert: nil converter")
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("convert: no benchmark samples")
	}
	if concurrentSlots <= 0 {
		concurrentSlots = 6
	}

	idleBaseline := SumX2TRSSBytes()
	out := make([]X2TMemorySample, 0, len(samples)+1)
	for label, samplePath := range samples {
		cacheDir, err := os.MkdirTemp("", "go-office-x2tmem-*")
		if err != nil {
			return nil, err
		}

		baseline, peak, err := MeasurePeakX2TRSS(ctx, 25*time.Millisecond, func() error {
			return conv.ToEditorBin(ctx, samplePath, cacheDir)
		})
		_ = os.RemoveAll(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("%s ToEditorBin: %w", label, err)
		}
		out = append(out, X2TMemorySample{
			Label:             label,
			BaselineRSSBytes:  baseline,
			PeakRSSBytes:      peak,
			PeakDeltaRSSBytes: peak - baseline,
		})
	}

	// Concurrent forward opens across formats.
	paths := make([]string, 0, len(samples))
	for _, p := range samples {
		paths = append(paths, p)
	}
	limitConv, err := New(Options{AssetDir: assetDir, Limit: concurrentSlots})
	if err != nil {
		return nil, err
	}

	var peakConcurrent int64
	var baselineConcurrent int64
	baselineConcurrent, peakConcurrent, err = MeasurePeakX2TRSS(ctx, 25*time.Millisecond, func() error {
		var wg sync.WaitGroup
		errCh := make(chan error, len(paths))
		for i, samplePath := range paths {
			wg.Add(1)
			go func(idx int, path string) {
				defer wg.Done()
				cacheDir, mkdirErr := os.MkdirTemp("", fmt.Sprintf("go-office-x2tmem-%d-*", idx))
				if mkdirErr != nil {
					errCh <- mkdirErr
					return
				}
				defer os.RemoveAll(cacheDir)
				if convertErr := limitConv.ToEditorBin(ctx, path, cacheDir); convertErr != nil {
					errCh <- convertErr
				}
			}(i, samplePath)
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("concurrent ToEditorBin: %w", err)
	}
	out = append(out, X2TMemorySample{
		Label:             fmt.Sprintf("concurrent-%d", concurrentSlots),
		BaselineRSSBytes:  baselineConcurrent,
		PeakRSSBytes:      peakConcurrent,
		PeakDeltaRSSBytes: peakConcurrent - baselineConcurrent,
		ConcurrentSlots:   concurrentSlots,
	})
	out = append(out, X2TMemorySample{
		Label:            "idle-baseline",
		BaselineRSSBytes: idleBaseline,
		PeakRSSBytes:     idleBaseline,
	})

	return out, nil
}
