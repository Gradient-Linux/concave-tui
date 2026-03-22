package model

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readMemInfo() (used uint64, total uint64, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}

	values := make(map[string]uint64)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		values[key] = value * 1024
	}

	total = values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
	}
	if available > total {
		available = total
	}
	return total - available, total, nil
}

func humanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}

	suffixes := []string{"KB", "MB", "GB", "TB", "PB"}
	size := float64(value)
	idx := -1
	for size >= unit && idx < len(suffixes)-1 {
		size /= unit
		idx++
	}
	if idx <= 0 {
		return fmt.Sprintf("%.0f %s", size, suffixes[max(0, idx)])
	}
	if size >= 10 {
		return fmt.Sprintf("%.0f %s", size, suffixes[idx])
	}
	return fmt.Sprintf("%.1f %s", size, suffixes[idx])
}
