package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
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
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metrics", metricsHandler)
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Println("metrics server listening on :8080")
	log.Fatal(srv.ListenAndServe())
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("os.Hostname: %v", err)
		hostname = "unknown"
	}
	b, err := json.Marshal(healthResponse{Status: "ok", Hostname: hostname})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("os.Hostname: %v", err)
		hostname = "unknown"
	}
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
	b, err := json.Marshal(metricsResponse{
		Hostname:        hostname,
		CPUUsagePercent: cpu,
		MemoryUsedMB:    memUsed,
		MemoryTotalMB:   memTotal,
		DiskUsedGB:      diskUsed,
		DiskTotalGB:     diskTotal,
		UptimeSeconds:   uptime,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
