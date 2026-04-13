# Zabbix Agent Simulator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal Alpine Docker image running Zabbix Agent 2 (active mode) and a real-metrics HTTP API, deployable at scale on the local k3d `zabbix` cluster where each pod auto-registers as a unique Zabbix host.

**Architecture:** Multi-stage Dockerfile — Go binary compiled in `golang:alpine`, copied into `alpine:latest` alongside `zabbix-agent2` from the Alpine community repo. An entrypoint shell script generates the agent config at runtime (injecting the pod name as hostname) then starts both processes. Images are loaded into k3d with `k3d image import` — no in-cluster registry needed.

**Tech Stack:** Go 1.22 (stdlib only), Alpine 3.19, zabbix-agent2 (Alpine community), k3d, kubectl, GNU Make.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `go.mod` | Create | Go module definition |
| `metrics.go` | Create | Proc reading + pure parsing functions |
| `server.go` | Create | main, HTTP handlers |
| `server_test.go` | Create | Parse function unit tests + HTTP handler tests |
| `entrypoint.sh` | Create | Runtime config generation + process supervisor |
| `Dockerfile` | Create | Multi-stage build |
| `Makefile` | Create | build / import / deploy / scale / status |
| `manifests/deployment.yaml` | Create | Deployment + Service |

---

## Task 1: Go module and metrics parsing

**Files:**
- Create: `/Users/kirk/Development/zabbix-agent-sim/go.mod`
- Create: `/Users/kirk/Development/zabbix-agent-sim/metrics.go`
- Create: `/Users/kirk/Development/zabbix-agent-sim/server_test.go`

- [ ] **Step 1: Create go.mod**

```
module zabbix-agent-sim

go 1.22
```

- [ ] **Step 2: Write failing tests for parsing functions**

Create `server_test.go`:

```go
package main

import (
	"testing"
	"os"
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
	wantTotal := uint64(1024000 / 1024)       // 1000 MB
	wantUsed := uint64((1024000 - 600000) / 1024) // 414 MB
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

// ── HTTP handlers (require /proc) ─────────────────────────────────────────────

func TestHealthHandler(t *testing.T) {
	if _, err := os.Stat("/proc"); err != nil {
		t.Skip("skipping: /proc not available (not Linux)")
	}
	// implemented in Task 2
}

func TestMetricsHandler(t *testing.T) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		t.Skip("skipping: /proc/stat not available (not Linux)")
	}
	// implemented in Task 2
}
```

- [ ] **Step 3: Run tests — expect compile failure (functions not defined)**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
go test ./...
```

Expected: `undefined: parseCPUSample` (or similar compile error)

- [ ] **Step 4: Create metrics.go**

```go
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
	idle1 := s1.idle + s1.iowait
	idle2 := s2.idle + s2.iowait
	total1 := s1.user + s1.nice + s1.system + idle1 + s1.irq + s1.softirq + s1.steal
	total2 := s2.user + s2.nice + s2.system + idle2 + s2.irq + s2.softirq + s2.steal
	totalDelta := total2 - total1
	idleDelta := idle2 - idle1
	if totalDelta == 0 {
		return 0, nil
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100.0, nil
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
```

- [ ] **Step 5: Run tests — expect parse tests to pass, handler tests skipped on macOS**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
go test ./... -v 2>&1 | grep -E "PASS|FAIL|SKIP|---"
```

Expected: `TestParseCPUSample_*`, `TestParseMemory_*`, `TestParseUptime_*` → PASS. Handler tests → SKIP.

- [ ] **Step 6: Commit**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
git add go.mod metrics.go server_test.go
git commit -m "feat: add metrics parsing with tests"
```

---

## Task 2: HTTP server

**Files:**
- Create: `/Users/kirk/Development/zabbix-agent-sim/server.go`
- Modify: `/Users/kirk/Development/zabbix-agent-sim/server_test.go` (fill in handler tests)

- [ ] **Step 1: Fill in the HTTP handler tests in server_test.go**

Replace the `TestHealthHandler` and `TestMetricsHandler` bodies:

```go
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
```

Add the missing imports to `server_test.go` (top of file):

```go
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)
```

- [ ] **Step 2: Run tests — expect compile failure (types not defined)**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
go test ./...
```

Expected: `undefined: healthHandler`, `undefined: healthResponse`, `undefined: metricsResponse`

- [ ] **Step 3: Create server.go**

```go
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
```

- [ ] **Step 4: Run tests — all parse tests pass, handler tests skipped on macOS**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
go test ./... -v 2>&1 | grep -E "PASS|FAIL|SKIP|---"
```

Expected: 7 PASS, 2 SKIP (handlers). 0 FAIL.

- [ ] **Step 5: Commit**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
git add server.go server_test.go
git commit -m "feat: add HTTP metrics server"
```

---

## Task 3: Entrypoint and Dockerfile

**Files:**
- Create: `/Users/kirk/Development/zabbix-agent-sim/entrypoint.sh`
- Create: `/Users/kirk/Development/zabbix-agent-sim/Dockerfile`

- [ ] **Step 1: Create entrypoint.sh**

```sh
#!/bin/sh
set -e

# Generate agent config at runtime — $HOSTNAME is injected by Kubernetes
# as the pod name via the HOSTNAME env var (fieldRef: metadata.name).
cat > /tmp/zabbix_agent2.conf << EOF
ServerActive=zabbix-zabbix-server.zabbix.svc.cluster.local
Hostname=${HOSTNAME}
HostMetadata=zabbix-agent-sim
LogType=console
EOF

# Start HTTP metrics server in the background
/usr/local/bin/metrics-server &

# Start Zabbix Agent 2 in the foreground (PID 1 after exec)
# When this process exits the container stops, taking metrics-server with it.
exec zabbix_agent2 -c /tmp/zabbix_agent2.conf
```

- [ ] **Step 2: Make entrypoint executable**

```bash
chmod +x /Users/kirk/Development/zabbix-agent-sim/entrypoint.sh
```

- [ ] **Step 3: Create Dockerfile**

```dockerfile
# ── Stage 1: Build the Go metrics server ──────────────────────────────────────
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod .
COPY metrics.go server.go ./
# Tests run here on Linux — handler tests will execute against real /proc
RUN go test ./... && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o metrics-server .

# ── Stage 2: Final image ───────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache zabbix-agent2 \
    --repository=https://dl-cdn.alpinelinux.org/alpine/v3.19/community
COPY --from=builder /build/metrics-server /usr/local/bin/metrics-server
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /usr/local/bin/metrics-server
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
```

- [ ] **Step 4: Build the image locally to verify**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
DOCKER_HOST=unix://$HOME/.colima/zabbix/docker.sock docker build -t zabbix-agent-sim:latest .
```

Expected: build succeeds, tests pass inside builder stage, final image created.

- [ ] **Step 5: Smoke-test the image locally**

```bash
DOCKER_HOST=unix://$HOME/.colima/zabbix/docker.sock \
  docker run --rm -e HOSTNAME=test-pod -p 8080:8080 zabbix-agent-sim:latest &
sleep 3
curl -s http://localhost:8080/health | python3 -m json.tool
curl -s http://localhost:8080/metrics | python3 -m json.tool
# Stop the container
DOCKER_HOST=unix://$HOME/.colima/zabbix/docker.sock docker stop $(docker ps -q --filter ancestor=zabbix-agent-sim:latest) 2>/dev/null || true
```

Expected: `/health` returns `{"status":"ok","hostname":"test-pod"}`. `/metrics` returns all six numeric fields.

- [ ] **Step 6: Commit**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
git add Dockerfile entrypoint.sh
git commit -m "feat: add Dockerfile and entrypoint"
```

---

## Task 4: Kubernetes manifest

**Files:**
- Create: `/Users/kirk/Development/zabbix-agent-sim/manifests/deployment.yaml`

- [ ] **Step 1: Create manifests/deployment.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zabbix-agent-sim
  namespace: zabbix
  labels:
    app: zabbix-agent-sim
spec:
  replicas: 1
  selector:
    matchLabels:
      app: zabbix-agent-sim
  template:
    metadata:
      labels:
        app: zabbix-agent-sim
    spec:
      containers:
      - name: zabbix-agent-sim
        image: zabbix-agent-sim:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 8080
          name: metrics
        env:
        - name: HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
---
apiVersion: v1
kind: Service
metadata:
  name: zabbix-agent-sim
  namespace: zabbix
  labels:
    app: zabbix-agent-sim
spec:
  selector:
    app: zabbix-agent-sim
  ports:
  - name: metrics
    port: 8080
    targetPort: 8080
```

- [ ] **Step 2: Commit**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
git add manifests/deployment.yaml
git commit -m "feat: add Kubernetes deployment manifest"
```

---

## Task 5: Makefile

**Files:**
- Create: `/Users/kirk/Development/zabbix-agent-sim/Makefile`

- [ ] **Step 1: Create Makefile**

```makefile
IMAGE_NAME   := zabbix-agent-sim
IMAGE_TAG    := latest
CLUSTER_NAME := zabbix
NAMESPACE    := zabbix
DOCKER_HOST  := unix://$(HOME)/.colima/$(CLUSTER_NAME)/docker.sock

export DOCKER_HOST

.PHONY: help build import deploy all scale status

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build the Docker image (runs Go tests inside builder stage)
	@DOCKER_HOST=$(DOCKER_HOST) docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

import: ## Import image into the k3d zabbix cluster
	@k3d image import $(IMAGE_NAME):$(IMAGE_TAG) --cluster $(CLUSTER_NAME)

deploy: ## Apply Kubernetes manifests
	@kubectl apply -f manifests/deployment.yaml

all: build import deploy ## Build, import, and deploy

scale: ## Scale to N replicas: make scale N=10
	@kubectl scale deployment/$(IMAGE_NAME) --replicas=$(N) -n $(NAMESPACE)

status: ## Show running pods and sample metrics from the first pod
	@echo "Pods:"
	@kubectl get pods -n $(NAMESPACE) -l app=$(IMAGE_NAME)
	@echo ""
	@echo "Sample metrics:"
	@POD=$$(kubectl get pods -n $(NAMESPACE) -l app=$(IMAGE_NAME) \
		-o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	 [ -n "$$POD" ] && kubectl exec -n $(NAMESPACE) $$POD -- \
		wget -qO- http://localhost:8080/metrics || echo "No pods running"
```

- [ ] **Step 2: Verify make help works**

```bash
cd /Users/kirk/Development/zabbix-agent-sim && make help
```

Expected: lists build, import, deploy, all, scale, status with descriptions.

- [ ] **Step 3: Commit**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
git add Makefile
git commit -m "feat: add Makefile"
```

---

## Task 6: Build, import, deploy and verify

**Prerequisites:** `k3d-zabbix` cluster running (`CLUSTER=zabbix make start` from k8s-colima-cluster).

- [ ] **Step 1: Build and import**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
make build
make import
```

Expected: image built, imported into k3d-zabbix cluster.

- [ ] **Step 2: Deploy**

```bash
make deploy
```

Expected: `deployment.apps/zabbix-agent-sim created`, `service/zabbix-agent-sim created`

- [ ] **Step 3: Verify pod is running**

```bash
make status
```

Expected: 1 pod in `Running` state, metrics JSON printed.

- [ ] **Step 4: Check agent registered in Zabbix**

Wait ~60 seconds for the active agent to connect, then check the Zabbix frontend:
**Monitoring → Latest data** — filter by host group or search for the pod name (e.g. `zabbix-agent-sim-xxxxx`).

If auto-registration action is not yet configured, the agent will appear in **Administration → Queue** but not yet as a host. Set up the auto-registration action (one-time):
1. Administration → General → Auto-registration → set to No encryption
2. Configuration → Actions → Autoregistration actions → Create action:
   - Name: `Register simulated agents`
   - Condition: Host metadata **contains** `zabbix-agent-sim`
   - Operations: Add host → Host groups: `Simulated Agents` (create if needed) → Link template: `Linux by Zabbix agent active`

- [ ] **Step 5: Scale to 5 and verify all register**

```bash
make scale N=5
kubectl get pods -n zabbix -l app=zabbix-agent-sim --watch
```

Wait for all 5 pods Running, then check Zabbix frontend — 5 unique hosts should appear under the `Simulated Agents` host group within ~60 seconds.

- [ ] **Step 6: Final commit**

```bash
cd /Users/kirk/Development/zabbix-agent-sim
git status  # should be clean
```

If clean, all done. If any files were modified during debugging, commit them:

```bash
git add -A && git commit -m "fix: post-deploy corrections"
```
