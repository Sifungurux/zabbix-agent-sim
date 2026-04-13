package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type cpuStats struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

// parseCPUSample parses the aggregate "cpu" line from /proc/stat content.
func parseCPUSample(content string) (cpuStats, error) {
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			return cpuStats{}, fmt.Errorf("unexpected cpu line format: %q", line)
		}
		var s cpuStats
		ptrs := []*uint64{&s.user, &s.nice, &s.system, &s.idle, &s.iowait, &s.irq, &s.softirq, &s.steal}
		for i, ptr := range ptrs {
			v, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				return cpuStats{}, fmt.Errorf("parse field %d: %w", i+1, err)
			}
			*ptr = v
		}
		return s, nil
	}
	return cpuStats{}, fmt.Errorf("cpu line not found in /proc/stat content")
}

// cpuPercent computes CPU usage percentage from two consecutive cpuStats samples.
// Returns 0 if total counters did not increase or if idle regressed (counter underflow).
func cpuPercent(s1, s2 cpuStats) float64 {
	idle1 := s1.idle + s1.iowait
	idle2 := s2.idle + s2.iowait
	total1 := s1.user + s1.nice + s1.system + idle1 + s1.irq + s1.softirq + s1.steal
	total2 := s2.user + s2.nice + s2.system + idle2 + s2.irq + s2.softirq + s2.steal
	if total2 <= total1 {
		return 0
	}
	if idle2 < idle1 {
		return 0
	}
	delta := total2 - total1
	return float64(delta-(idle2-idle1)) / float64(delta) * 100.0
}

// readCPU samples /proc/stat twice 100ms apart and returns CPU usage percent.
func readCPU() (float64, error) {
	sample := func() (cpuStats, error) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return cpuStats{}, err
		}
		return parseCPUSample(string(data))
	}
	s1, err := sample()
	if err != nil {
		return 0, err
	}
	time.Sleep(100 * time.Millisecond)
	s2, err := sample()
	if err != nil {
		return 0, err
	}
	return cpuPercent(s1, s2), nil
}

// parseMemory parses /proc/meminfo content and returns (usedMB, totalMB).
func parseMemory(content string) (usedMB, totalMB uint64, err error) {
	var totalKB, availableKB uint64
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			totalKB = v
		case "MemAvailable:":
			availableKB = v
		}
	}
	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found in /proc/meminfo content")
	}
	return (totalKB - availableKB) / 1024, totalKB / 1024, nil
}

// readMemory reads /proc/meminfo and returns (usedMB, totalMB).
func readMemory() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	return parseMemory(string(data))
}

// readDisk returns (usedGB, totalGB) for the root filesystem.
func readDisk() (usedGB, totalGB float64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, 0, err
	}
	total := float64(stat.Blocks) * float64(stat.Bsize) / 1e9
	free := float64(stat.Bfree) * float64(stat.Bsize) / 1e9
	return total - free, total, nil
}

// parseUptime parses /proc/uptime content and returns uptime in seconds.
func parseUptime(content string) (float64, error) {
	fields := strings.Fields(strings.TrimSpace(content))
	if len(fields) < 1 || fields[0] == "" {
		return 0, fmt.Errorf("unexpected /proc/uptime format: %q", content)
	}
	return strconv.ParseFloat(fields[0], 64)
}

// readUptime reads /proc/uptime and returns uptime in seconds.
func readUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	return parseUptime(string(data))
}
