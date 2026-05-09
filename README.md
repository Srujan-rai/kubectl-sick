# kubectl-sick

Lists every broken thing in your cluster, sorted worst-first.

```
SEVERITY   KIND         NAMESPACE    NAME                        REASON
CRITICAL   Pod          payments     api-7d9f8b-xkp2r            CrashLoopBackOff (14x)
CRITICAL   Node         <none>       node-3                      NotReady
CRITICAL   PVC          staging      postgres-data               Pending (unbound) 23m
WARNING    Pod          payments     worker-6d4c9-zzx8k          OOMKilled
WARNING    Deployment   monitoring   prometheus                  2/3 replicas available
WARNING    Job          crons        nightly-report-28392        Failed (backoffLimit reached)
WARNING    Certificate  default      api-tls                     Expiring in 4d
INFO       HPA          production   checkout-hpa                At max replicas 38m

── 3 critical · 4 warning · 1 info ──────────────────────────────────────
```

## Install

### Manual
Download the binary for your platform from [releases](https://github.com/srujan-rai/kubectl-sick/releases), then:
```bash
chmod +x kubectl-sick
mv kubectl-sick /usr/local/bin/kubectl-sick
```

### From source
```bash
git clone https://github.com/srujan-rai/kubectl-sick
cd kubectl-sick
make install
```

### krew
```bash
kubectl krew install sick
```

## Usage

```bash
kubectl sick                              # scan entire cluster
kubectl sick -n payments                  # scope to one namespace
kubectl sick --min-severity warning       # hide INFO
kubectl sick --exit-code                  # exit 1 on any CRITICAL (CI use)
kubectl sick --json                       # machine-readable JSON
kubectl sick --yaml                       # machine-readable YAML
kubectl sick --watch --interval 15        # live diff, refresh every 15s
kubectl sick --explain payments/api-xyz   # show logs + events for a resource
kubectl sick --context prod-us-east       # use a specific kubeconfig context
kubectl sick --no-color                   # plain output (pipes, logs)
```

## What it checks

| Resource | Detected |
|----------|---------|
| **Pods** | CrashLoopBackOff, OOMKilled, ImagePullBackOff, Error, Pending >5m, stuck terminating >10m |
| **Nodes** | NotReady, MemoryPressure, DiskPressure, PIDPressure |
| **PVCs** | Pending (unbound), Lost |
| **Deployments** | Unavailable replicas (0 = CRITICAL, partial = WARNING) |
| **Jobs** | Failed (backoffLimit reached), running >1h |
| **Certificates** | Expiring <7d, expired, not ready — silently skipped if cert-manager absent |
| **HPAs** | At maxReplicas for >30 minutes |

## Severity

| Level | Color | Meaning |
|-------|-------|---------|
| CRITICAL | red | Service likely down or degraded |
| WARNING | yellow | Degraded, needs attention |
| INFO | blue | Noteworthy, not urgent |

## --explain

Fetches logs and events for a specific broken resource:

```bash
kubectl sick --explain payments/api-7d9f8b-xkp2r
# or with explicit kind:
kubectl sick --explain pod/payments/api-7d9f8b-xkp2r
```

```
── Pod: payments/api-7d9f8b-xkp2r ─────────────────────────────────────

LOGS (last 20 lines):
  [container: api]
  2024-01-15T10:23:41Z ERROR failed to connect to database: connection refused
  2024-01-15T10:23:41Z ERROR retrying in 5s (attempt 14/∞)

EVENTS (last 5 warnings):
  10m   Warning   BackOff   pod/api-7d9f8b-xkp2r   Back-off restarting failed container
```

## --watch mode

```bash
kubectl sick --watch --interval 15
```

Clears the terminal between scans. New issues are marked `[NEW]`, resolved ones appear briefly as `[RESOLVED]`. Exit with Ctrl+C.

## JSON output

```bash
kubectl sick --json
```

```json
{
  "scanned_at": "2024-01-15T14:32:07Z",
  "cluster_context": "prod-us-east",
  "summary": { "critical": 2, "warning": 4, "info": 1 },
  "issues": [
    {
      "severity": "CRITICAL",
      "kind": "Pod",
      "namespace": "payments",
      "name": "api-7d9f8b-xkp2r",
      "reason": "CrashLoopBackOff (14x)",
      "since": "2024-01-15T10:23:41Z"
    }
  ]
}
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --namespace` | all | Scope scan to a namespace |
| `--min-severity` | `info` | Minimum severity: `info`, `warning`, `critical` |
| `--json` | false | JSON output |
| `--yaml` | false | YAML output |
| `--exit-code` | false | Exit 1 if any CRITICAL found |
| `--explain` | — | Logs + events for `namespace/name` or `kind/namespace/name` |
| `--watch` | false | Live scan with diff |
| `--interval` | 30 | Seconds between scans (with `--watch`) |
| `--no-color` | false | Disable ANSI colors |
| `--context` | current | Kubeconfig context |
| `--kubeconfig` | `~/.kube/config` | Path to kubeconfig |

## Build

```bash
make build          # bin/kubectl-sick
make test           # run tests
make plugin-install # copy to $GOPATH/bin
```

## Requirements

- Go 1.21+
- Kubernetes cluster access via kubeconfig
- cert-manager is optional — silently skipped if not installed
