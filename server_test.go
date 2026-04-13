package main

import (
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

// ── HTTP handlers (require /proc — implemented in Task 2) ─────────────────────

func TestHealthHandler(t *testing.T) {
	if _, err := os.Stat("/proc"); err != nil {
		t.Skip("skipping: /proc not available (not Linux)")
	}
	// TODO: filled in Task 2
}

func TestMetricsHandler(t *testing.T) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		t.Skip("skipping: /proc/stat not available (not Linux)")
	}
	// TODO: filled in Task 2
}
