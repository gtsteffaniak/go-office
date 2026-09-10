//go:build linux

package runlimit

import (
	"math"
	"os"
	"strconv"
	"strings"
)

// Resources describes cgroup or host CPU and memory limits visible to this process.
type Resources struct {
	MemoryLimitBytes int64
	MemoryCurrentBytes int64
	CPUs             float64
}

// ReadResources reads cgroup v2 (preferred) or v1 limits, falling back to /proc/meminfo.
func ReadResources() Resources {
	res := Resources{
		MemoryLimitBytes: memoryLimitBytes(),
		CPUs:             cpuLimit(),
	}
	res.MemoryCurrentBytes = memoryCurrentBytes()
	return res
}

func memoryLimitBytes() int64 {
	for _, path := range []string{
		"/sys/fs/cgroup/memory.max",
		"/sys/fs/cgroup/memory/memory.limit_in_bytes",
	} {
		if limit := readInt64File(path); limit > 0 && !isUnlimitedMemoryLimit(limit) {
			return limit
		}
	}
	return hostMemoryBytes()
}

func isUnlimitedMemoryLimit(limit int64) bool {
	// cgroup v1 uses a very large sentinel for "no limit".
	const unlimited = 9223372036854771712
	return limit >= unlimited || limit > 1<<62
}

func memoryCurrentBytes() int64 {
	for _, path := range []string{
		"/sys/fs/cgroup/memory.current",
		"/sys/fs/cgroup/memory/memory.usage_in_bytes",
	} {
		if current := readInt64File(path); current > 0 {
			return current
		}
	}
	return 0
}

func cpuLimit() float64 {
	parts := strings.Fields(readStringFile("/sys/fs/cgroup/cpu.max"))
	if len(parts) == 2 && parts[0] != "max" {
		quota, err1 := strconv.ParseInt(parts[0], 10, 64)
		period, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 == nil && err2 == nil && quota > 0 && period > 0 {
			return float64(quota) / float64(period)
		}
	}
	if quota := readInt64File("/sys/fs/cgroup/cpu/cpu.cfs_quota_us"); quota > 0 {
		period := readInt64File("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
		if period > 0 {
			return float64(quota) / float64(period)
		}
	}
	return float64(runtimeCPUCount())
}

func hostMemoryBytes() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
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

func runtimeCPUCount() int {
	n := 0
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "processor") {
			n++
		}
	}
	if n == 0 {
		return 1
	}
	return n
}

func readInt64File(path string) int64 {
	raw := strings.TrimSpace(readStringFile(path))
	if raw == "" || raw == "max" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func readStringFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// EffectiveMemoryBudget returns bytes available for Playwright + x2t after reserving
// headroom for the OS, go-office, and Node tooling.
func EffectiveMemoryBudget(res Resources) int64 {
	limit := res.MemoryLimitBytes
	if limit <= 0 {
		return 0
	}
	headroom := int64(math.Max(256*1024*1024, float64(limit)*0.12))
	budget := limit - headroom
	if budget < 512*1024*1024 {
		return budget
	}
	return budget
}
