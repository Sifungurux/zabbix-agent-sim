# Zabbix Agent Simulator Design

**Date:** 2026-04-13  
**Repo:** `~/Development/zabbix-agent-sim`  
**Goal:** A minimal Alpine Docker image that simulates a monitored server — runs Zabbix Agent 2 and a real-metrics HTTP API — deployable at scale (200+ pods) where each pod appears as a unique host in Zabbix.

---

## Use Case

Testing Zabbix monitoring at scale in the local k3d `zabbix` cluster. Deploy N replicas, each auto-registers with the Zabbix server as a distinct host with real pod metrics. Scale up to stress-test dashboards, triggers, and data collection. Scale down to clean up.

---

## Image

**Base:** `alpine:latest` (minimal CVE surface)  
**Build:** Multi-stage Dockerfile

| Stage | Image | Purpose |
|---|---|---|
| builder | `golang:alpine` | Compile the HTTP metrics server binary |
| final | `alpine:latest` | Install zabbix-agent2 + copy binary |

### Final image contents

- `zabbix-agent2` (from Alpine community repo)
- Compiled Go HTTP server binary (`/usr/local/bin/metrics-server`)
- Agent config (`/etc/zabbix/zabbix_agent2.conf`)
- Entrypoint shell script (`/entrypoint.sh`) — starts both processes

---

## Repository Structure

```
zabbix-agent-sim/
├── Dockerfile
├── Makefile
├── server.go                  # Go HTTP metrics server
├── zabbix_agent2.conf         # Zabbix Agent 2 config template
├── entrypoint.sh              # Starts agent + HTTP server
└── manifests/
    └── deployment.yaml        # Kubernetes Deployment + Service
```

---

## HTTP Server (`server.go`)

Listens on port `8080`. Reads real values from the Linux kernel at request time.

### `GET /health`

```json
{"status": "ok", "hostname": "<pod-name>"}
```

### `GET /metrics`

```json
{
  "hostname": "<pod-name>",
  "cpu_usage_percent": 12.4,
  "memory_used_mb": 38,
  "memory_total_mb": 512,
  "disk_used_gb": 1.2,
  "disk_total_gb": 20.0,
  "uptime_seconds": 4821
}
```

**Data sources (all read from kernel, no dependencies):**

| Field | Source |
|---|---|
| `cpu_usage_percent` | `/proc/stat` — 1-second sample delta |
| `memory_used_mb` / `memory_total_mb` | `/proc/meminfo` |
| `disk_used_gb` / `disk_total_gb` | `syscall.Statfs` on `/` |
| `uptime_seconds` | `/proc/uptime` |
| `hostname` | `os.Hostname()` (= pod name in k8s) |

---

## Zabbix Agent Config

Passive checks (server polls) + active checks (agent connects to server for auto-registration):

```
Server=0.0.0.0/0
ServerActive=zabbix-zabbix-server.zabbix.svc.cluster.local
Hostname=${HOSTNAME}
HostMetadata=zabbix-agent-sim
ListenPort=10050
LogType=console
```

- `${HOSTNAME}` is the k8s pod name — unique per replica
- `HostMetadata=zabbix-agent-sim` is the filter used in the Zabbix auto-registration action
- Active mode enables auto-registration; passive mode enables item polling

---

## Kubernetes Deployment

`manifests/deployment.yaml` contains a `Deployment` and a `Service`.

**Deployment:**
- `replicas: 1` (scale with `kubectl scale deployment/zabbix-agent-sim --replicas=200 -n zabbix`)
- Image: `registry.registry.svc.cluster.local:5000/zabbix-agent-sim:latest`
- Namespace: `zabbix` (same namespace as the Zabbix stack)
- `HOSTNAME` env var injected via `valueFrom.fieldRef.fieldPath: metadata.name`

**Service:**
- Type: `ClusterIP`
- Port `10050` — Zabbix passive agent checks
- Port `8080` — HTTP metrics API

---

## Makefile Targets

| Target | Description |
|---|---|
| `make build` | Build image via Colima Docker socket |
| `make push` | Push to in-cluster registry (`registry.registry.svc.cluster.local:5000`) |
| `make deploy` | `kubectl apply` the deployment manifest |
| `make all` | build + push + deploy |
| `make scale N=<n>` | Scale to N replicas |
| `make status` | Show pod count and a sample pod's metrics |

---

## Zabbix Auto-Registration Setup (manual, one-time)

After first deploy, configure in Zabbix frontend:

1. **Administration → General → Auto-registration** — set encryption to `No encryption`
2. **Configuration → Actions → Auto-registration actions → Create action:**
   - Condition: `Host metadata contains zabbix-agent-sim`
   - Operations: Add host, add to host group `Simulated Agents`, link template `Linux by Zabbix agent`

All pods matching the metadata string will be automatically added as hosts with the Linux template applied.

---

## Registry

Uses the same in-cluster Docker registry already deployed in the `registry` namespace (`registry.registry.svc.cluster.local:5000`). Build and push from the Mac via Colima's Docker socket. The k3d cluster trusts this registry via its `registries.yaml` config.

---

## Out of Scope

- TLS between agent and server
- Custom Zabbix user parameters
- Persistent storage
