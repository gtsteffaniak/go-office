//go:build linux

package runlimit

import (
	"testing"
)

func TestReadResources(t *testing.T) {
	res := ReadResources()
	if res.CPUs <= 0 {
		t.Fatalf("expected positive CPUs, got %v", res.CPUs)
	}
	if res.MemoryLimitBytes < 0 {
		t.Fatalf("negative memory limit: %d", res.MemoryLimitBytes)
	}
	t.Logf("memory_limit=%d memory_current=%d cpus=%.2f", res.MemoryLimitBytes, res.MemoryCurrentBytes, res.CPUs)
}

func TestRecommendLimitsWithMeasuredProfile(t *testing.T) {
	profile := X2TMemoryProfile{
		BaselineRSSBytes:       0,
		SinglePeakRSSBytes:     96899072,
		ConcurrentPeakRSSBytes: 324845568,
		ConcurrentSlots:        6,
	}
	res := Resources{
		MemoryLimitBytes: 8 * 1024 * 1024 * 1024,
		CPUs:             4,
	}
	limits := RecommendLimits(res, profile)
	if limits.ConvertLimit != 4 {
		t.Fatalf("expected convert limit 4 on 8GiB host, got %d", limits.ConvertLimit)
	}
	if limits.PlaywrightWorkers < 1 || limits.PlaywrightWorkers > 10 {
		t.Fatalf("unexpected workers: %d", limits.PlaywrightWorkers)
	}
}
