package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// ── parseCPUSample ────────────────────────────────────────────────────────────

func TestParseCPUSample_ValidInput(t *testing.T) {
	input := "cpu  1000 200 300 4000 100 50 25 10\ncpu0 500 100 150 2000 50 25 12 5\n"
	s, err := parseCPUSample(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.user != 1000 {
		t.Errorf("user: got %d, want 1000", s.user)
	}
	if s.nice != 200 {
		t.Errorf("nice: got %d, want 200", s.nice)
	}
	if s.idle != 4000 {
		t.Errorf("idle: got %d, want 4000", s.idle)
	}
	if s.steal != 10 {
		t.Errorf("steal: got %d, want 10", s.steal)
	}
}

func TestParseCPUSample_NoCPULine(t *testing.T) {
	_, err := parseCPUSample("no cpu here\n")
	if err == nil {
		t.Error("expected error for missing cpu line")
	}
}

func TestParseCPUSample_ShortLine(t *testing.T) {
	_, err := parseCPUSample("cpu  1000 200\n")
	if err == nil {
		t.Error("expected error for short cpu line")
	}
}

// ── parseMemory ───────────────────────────────────────────────────────────────

func TestParseMemory_ValidInput(t *testing.T) {
	input := "MemTotal:       1024000 kB\nMemFree:        512000 kB\nMemAvailable:   600000 kB\n"
	used, total, err := parseMemory(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTotal := uint64(1024000 / 1024)
	wantUsed := uint64((1024000 - 600000) / 1024)
	if total != wantTotal {
		t.Errorf("total: got %d MB, want %d", total, wantTotal)
	}
	if used != wantUsed {
		t.Errorf("used: got %d MB, want %d", used, wantUsed)
	}
}

func TestParseMemory_NoMemTotal(t *testing.T) {
	_, _, err := parseMemory("MemFree: 1000 kB\n")
	if err == nil {
		t.Error("expected error for missing MemTotal")
	}
}

// ── parseUptime ───────────────────────────────────────────────────────────────

func TestParseUptime_ValidInput(t *testing.T) {
	uptime, err := parseUptime("12345.67 890.12\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uptime != 12345.67 {
		t.Errorf("got %f, want 12345.67", uptime)
	}
}

func TestParseUptime_EmptyInput(t *testing.T) {
	_, err := parseUptime("   \n")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// ── cpuPercent ────────────────────────────────────────────────────────────────

func TestCPUPercent_FullyBusy(t *testing.T) {
	s1 := cpuStats{user: 1000, idle: 1000}
	s2 := cpuStats{user: 2000, idle: 1000} // all delta is user (non-idle)
	got := cpuPercent(s1, s2)
	if got != 100.0 {
		t.Errorf("got %f, want 100.0", got)
	}
}

func TestCPUPercent_FullyIdle(t *testing.T) {
	s1 := cpuStats{user: 1000, idle: 1000}
	s2 := cpuStats{user: 1000, idle: 2000} // all delta is idle
	got := cpuPercent(s1, s2)
	if got != 0.0 {
		t.Errorf("got %f, want 0.0", got)
	}
}

func TestCPUPercent_ZeroDelta(t *testing.T) {
	s1 := cpuStats{user: 1000, idle: 1000}
	s2 := cpuStats{user: 1000, idle: 1000} // no change
	got := cpuPercent(s1, s2)
	if got != 0.0 {
		t.Errorf("got %f, want 0.0", got)
	}
}

// ── HTTP handlers (require /proc — implemented in Task 2) ─────────────────────

func TestHealthHandler(t *testing.T) {
	if _, err := os.Stat("/proc"); err != nil {
		t.Skip("skipping: /proc not available (not Linux)")
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	healthHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status: got %q, want \"ok\"", resp.Status)
	}
	if resp.Hostname == "" {
		t.Error("hostname must not be empty")
	}
}

func TestMetricsHandler(t *testing.T) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		t.Skip("skipping: /proc/stat not available (not Linux)")
	}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metricsHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp metricsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.CPUUsagePercent < 0 || resp.CPUUsagePercent > 100 {
		t.Errorf("cpu_usage_percent out of range: %f", resp.CPUUsagePercent)
	}
	if resp.MemoryTotalMB == 0 {
		t.Error("memory_total_mb must not be zero")
	}
	if resp.DiskTotalGB == 0 {
		t.Error("disk_total_gb must not be zero")
	}
	if resp.UptimeSeconds <= 0 {
		t.Error("uptime_seconds must be positive")
	}
	if resp.Hostname == "" {
		t.Error("hostname must not be empty")
	}
}
