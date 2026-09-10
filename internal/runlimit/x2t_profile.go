package runlimit

// X2TMemoryProfile holds measured x2t RSS peaks used to size OFFICE_CONVERT_LIMIT.
// Values are updated by TestMeasureX2TMemoryProfile and cmd/measure-x2t-memory.
type X2TMemoryProfile struct {
	BaselineRSSBytes       int64
	SinglePeakRSSBytes     int64
	ConcurrentPeakRSSBytes int64
	ConcurrentSlots        int
}

// DefaultX2TMemoryProfile is populated after running the memory benchmark.
// Placeholder zeros fall back to conservative per-slot estimates in RecommendLimits.
var DefaultX2TMemoryProfile = X2TMemoryProfile{
	BaselineRSSBytes:       0,
	SinglePeakRSSBytes:     96112640,
	ConcurrentPeakRSSBytes: 318930944,
	ConcurrentSlots:        6,
}

// PerSlotRSS returns the measured bytes per active x2t slot at ConcurrentSlots.
func (p X2TMemoryProfile) PerSlotRSS() int64 {
	if p.ConcurrentSlots <= 0 {
		p.ConcurrentSlots = 6
	}
	if p.ConcurrentPeakRSSBytes > p.BaselineRSSBytes {
		return (p.ConcurrentPeakRSSBytes - p.BaselineRSSBytes) / int64(p.ConcurrentSlots)
	}
	if p.SinglePeakRSSBytes > p.BaselineRSSBytes {
		return p.SinglePeakRSSBytes - p.BaselineRSSBytes
	}
	return 0
}
