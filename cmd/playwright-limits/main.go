//go:build linux

package main

import (
	"fmt"

	"github.com/quantumx-apps/go-office/internal/runlimit"
)

func main() {
	res := runlimit.ReadResources()
	limits := runlimit.RecommendLimits(res, runlimit.DefaultX2TMemoryProfile)
	fmt.Printf("PLAYWRIGHT_WORKERS=%d\n", limits.PlaywrightWorkers)
	fmt.Printf("OFFICE_CONVERT_LIMIT=%d\n", limits.ConvertLimit)
	fmt.Printf("# memory_budget_bytes=%d per_slot_rss_bytes=%d cpus=%.2f\n",
		limits.MemoryBudgetBytes, limits.PerSlotRSSBytes, limits.CPUs)
}
