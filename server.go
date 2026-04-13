package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type healthResponse struct {
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
}

type metricsResponse struct {
	Hostname        string  `json:"hostname"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	MemoryUsedMB    uint64  `json:"memory_used_mb"`
	MemoryTotalMB   uint64  `json:"memory_total_mb"`
	DiskUsedGB      float64 `json:"disk_used_gb"`
	DiskTotalGB     float64 `json:"disk_total_gb"`
	UptimeSeconds   float64 `json:"uptime_seconds"`
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/metrics", metricsHandler)
	log.Println("metrics server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{Status: "ok", Hostname: hostname})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	cpu, err := readCPU()
	if err != nil {
		http.Error(w, "failed to read cpu: "+err.Error(), http.StatusInternalServerError)
		return
	}
	memUsed, memTotal, err := readMemory()
	if err != nil {
		http.Error(w, "failed to read memory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	diskUsed, diskTotal, err := readDisk()
	if err != nil {
		http.Error(w, "failed to read disk: "+err.Error(), http.StatusInternalServerError)
		return
	}
	uptime, err := readUptime()
	if err != nil {
		http.Error(w, "failed to read uptime: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metricsResponse{
		Hostname:        hostname,
		CPUUsagePercent: cpu,
		MemoryUsedMB:    memUsed,
		MemoryTotalMB:   memTotal,
		DiskUsedGB:      diskUsed,
		DiskTotalGB:     diskTotal,
		UptimeSeconds:   uptime,
	})
}
