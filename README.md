# zabbix-agent-sim

Alpine Docker image that simulates monitored servers at scale. Each pod runs Zabbix Agent 2 (active mode) and a real-metrics HTTP API. Deploy hundreds of replicas — each auto-registers as a unique host in Zabbix using the pod name as hostname.

## Quick start

```bash
make all        # build + import into k3d + deploy (1 replica)
make status     # pod list + live metrics from first pod
make scale N=20 # scale to 20 replicas
```

## Requirements

- k3d cluster named `zabbix` running
- Zabbix stack deployed in the `zabbix` namespace
- [One-time Zabbix setup](#one-time-zabbix-setup) completed

## Makefile targets

| Target | Description |
|---|---|
| `make build` | Build image (runs Go tests inside builder stage) |
| `make import` | Import image into the k3d-zabbix cluster |
| `make deploy` | Apply Kubernetes manifests |
| `make all` | build + import + deploy |
| `make scale N=<n>` | Scale to N replicas |
| `make status` | Show pods and live metrics from first pod |

## One-time Zabbix setup

Port-forward to the Zabbix frontend:

```bash
kubectl port-forward svc/zabbix-zabbix-web 8888:80 -n zabbix
```

**1. Disable auto-registration encryption:**

Open `http://localhost:8888/zabbix.php?action=autoreg.edit` → set to `No encryption` → Update

**2. Create auto-registration action:**

Open `http://localhost:8888/actionconf.php?eventsource=2` → **Create action**

| Field | Value |
|---|---|
| Name | `Register simulated agents` |
| Condition | Host metadata **contains** `zabbix-agent-sim` |
| Operations | Add host · Add to group `Simulated Agents` · Link template `Linux by Zabbix agent active` |

New pods register as Zabbix hosts within ~60 seconds.

## Metrics API

Each pod exposes real `/proc` metrics on port 8080:

```
GET /health   → {"status":"ok","hostname":"<pod-name>"}
GET /metrics  → {"hostname":"...","cpu_usage_percent":2.4,"memory_used_mb":1783,...}
```

Query from inside the cluster:

```bash
kubectl exec -n zabbix <pod-name> -- wget -qO- http://localhost:8080/metrics
```

## Architecture

```
Pod
├── zabbix_agent2        Active mode → pushes to zabbix-zabbix-server:10051
│                        Hostname = pod name (from metadata.name)
│                        HostMetadata = zabbix-agent-sim (triggers auto-registration)
└── metrics-server       HTTP on :8080
                         Reads /proc/stat, /proc/meminfo, /proc/uptime, syscall.Statfs
```

Multi-stage Dockerfile: Go binary compiled in `golang:1.22-alpine`, copied into `alpine:3.19` alongside `zabbix-agent2`. Image loaded into k3d with `k3d image import` — no registry needed.

## Troubleshooting

### Pods start but don't appear in Zabbix

1. Verify the auto-registration action exists and is enabled at `http://localhost:8888/actionconf.php?eventsource=2`
2. Check agent logs — look for connection errors to `zabbix-zabbix-server.zabbix.svc.cluster.local:10051`:

```bash
kubectl logs -n zabbix <pod-name>
```

### `ErrImageNeverPull` or `ImagePullBackOff`

Image wasn't imported into the cluster. Re-import and restart:

```bash
make import
kubectl rollout restart deployment/zabbix-agent-sim -n zabbix
```

### Pod crashes immediately (`CrashLoopBackOff`)

Check logs — if `HOSTNAME must be set` appears, the `fieldRef: metadata.name` env injection is missing from the deployment manifest.

```bash
kubectl logs -n zabbix <pod-name>
```

### Clean up

```bash
make scale N=0          # stop all pods (keeps deployment)
kubectl delete -f manifests/deployment.yaml  # remove deployment + service
```
