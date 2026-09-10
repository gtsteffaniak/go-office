package runlimit

import "math"

const (
	maxPlaywrightWorkers = 10
	maxConvertLimit      = 4
	// Chromium editor working set observed under Playwright load (conservative).
	chromiumWorkerRSSBytes = 380 * 1024 * 1024
	// Share of memory budget reserved for concurrent x2t conversions.
	x2tMemoryShare = 0.35
)

// Limits holds derived Playwright and converter concurrency caps.
type Limits struct {
	PlaywrightWorkers int
	ConvertLimit      int
	MemoryBudgetBytes int64
	PerSlotRSSBytes   int64
	CPUs              float64
}

// RecommendLimits sizes Playwright workers and OFFICE_CONVERT_LIMIT from cgroup
// resources and measured x2t memory profile.
func RecommendLimits(res Resources, prof X2TMemoryProfile) Limits {
	budget := EffectiveMemoryBudget(res)
	perSlot := prof.PerSlotRSS()
	if perSlot <= 0 {
		perSlot = 18 * 1024 * 1024 // conservative until benchmark populates profile
	}

	convertByMem := maxConvertLimit
	if budget > 0 {
		x2tBudget := int64(float64(budget) * x2tMemoryShare)
		if perSlot > 0 {
			convertByMem = int(math.Max(1, math.Min(float64(maxConvertLimit), math.Floor(float64(x2tBudget)/float64(perSlot)))))
		}
	}

	workersByCPU := int(math.Max(1, math.Min(float64(maxPlaywrightWorkers), math.Floor(res.CPUs*1.25))))
	workersByMem := maxPlaywrightWorkers
	if budget > 0 {
		remaining := budget - int64(convertByMem)*perSlot
		if remaining > 0 {
			workersByMem = int(math.Max(1, math.Min(float64(maxPlaywrightWorkers), math.Floor(float64(remaining)/float64(chromiumWorkerRSSBytes)))))
		} else {
			workersByMem = 1
		}
	}

	workers := workersByCPU
	if workersByMem < workers {
		workers = workersByMem
	}
	if workers < 1 {
		workers = 1
	}

	return Limits{
		PlaywrightWorkers: workers,
		ConvertLimit:      convertByMem,
		MemoryBudgetBytes: budget,
		PerSlotRSSBytes:   perSlot,
		CPUs:              res.CPUs,
	}
}
