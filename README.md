# ControlRoom

A modern, single-binary admin UI for headless Linux homelab servers — designed
to replace routine SSH for the things you do over and over.

> **Status:** v0.2 is merged and live on `main`. See [`CHANGELOG.md`](./CHANGELOG.md)
> for what shipped and [`MILESTONES.md`](./MILESTONES.md) for the roadmap.

## Features

| Tab | What it does | Backed by |
|---|---|---|
| **Dashboard** | Live CPU / memory / disks / temperatures / network rates / uptime / load (1 Hz over WebSocket). | `/proc`, `/sys` |
| **Updates** | `apt list --upgradable`, one-click check/apply with live job streaming, reboot-required banner. | `apt`, `sudo`, `systemctl` |
| **Services** | List / start / stop / restart / enable / disable systemd units; live `journalctl -fu` log tail. | dbus |
| **Containers** | Docker (and Podman socket-compatible) list with Compose-project grouping; per-container live logs + CPU/MEM stats; lifecycle actions. | `/var/run/docker.sock` |
| **Kubernetes** | Cluster management — list / detail / events for nodes, namespaces, workloads (Deployment/StatefulSet/DaemonSet), pods, services, configmaps, secrets. Pod log streaming. Pod **exec** in the browser (xterm.js + SPDY). Lifecycle actions: restart workload, scale, delete pod, cordon/uncordon node. ConfigMap structured key/value editor. Secret read-only viewer (masked-by-default, audited). Monaco-based **YAML editor** with server-side dry-run + conflict detection. | client-go |
| **Terminal** | Full PTY in the browser via xterm.js. Either a local shell or — in container deployments — a real host shell after PAM login. | `/dev/pty`, `nsenter`, `su` |
| **Network** | Read-only interfaces + UFW rules editor (add / delete / enable / disable). | `ip -j`, `ufw`, `sudo` |
| **Logs** | journald browser with unit / priority / since / search filters and live tail. Falls back to streaming docker container logs when journald isn't reachable. | `journalctl`, `/var/run/docker.sock` |
| **Settings** | Change password, manage 2FA, view server config + audit log. | SQLite |

## Three deployment shapes

| Shape | What runs where | Privilege model | When to pick |
|---|---|---|---|
| **Bare-metal** (`deploy/install.sh`) | Single Go binary as a hardened systemd unit; dedicated `controlroom` user; tight sudoers fragment for the privileged commands. | Unprivileged user + scoped sudoers. Compromise = whatever the sudoers fragment allows. | Recommended default. Smallest blast radius for the full feature set. |
| **Docker container** (`deploy/docker-compose.yml`) | Single container with `network_mode: host`, `pid: host`, `cap_add: SYS_ADMIN/SYS_PTRACE/NET_ADMIN`, host bind mounts for dbus / journal / apt / kubeconfig. Runs as root inside. | Effectively root on the host. | Single-user homelab where the operator already has root and wants one image to deploy. |
| **In-cluster Kubernetes Pod** (`deploy/k8s/`) | Manifests for namespace + ServiceAccount + ClusterRole (`get,list,watch` + `pods/log get`) + Deployment + Service + Ingress + PVC. Runs as nonroot uid 65532, drop ALL caps, readOnlyRootFilesystem. | Scoped RBAC. Cluster-only feature surface — **no host integrations** (no Services / Updates / Network / host Terminal). | Cluster-only ControlRoom. Pair with bare-metal or container ControlRoom on the host if you also want host management. |

Detailed install instructions for each: [`docs/INSTALL.md`](./docs/INSTALL.md).
Full env-var reference + per-tab capability matrix: [`docs/CONFIG.md`](./docs/CONFIG.md).
Per-shape threat model: [`docs/SECURITY.md`](./docs/SECURITY.md).

## Quick start

### Bare-metal (Debian / Ubuntu / Raspberry Pi OS)

```bash
sudo deploy/install.sh
sudo journalctl -u controlroom -n 200 | grep setup_token
```

### Docker container (host-privileged)

```bash
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml logs --tail 200 | grep setup_token
```

### In-cluster Kubernetes Pod

```bash
kubectl apply -f deploy/k8s/namespace.yaml \
              -f deploy/k8s/rbac.yaml \
              -f deploy/k8s/pvc.yaml \
              -f deploy/k8s/deployment.yaml \
              -f deploy/k8s/service.yaml
kubectl -n controlroom logs deployment/controlroom | grep setup_token
```

In all three shapes: open `https://<host>:8443`, paste the token from the
log, create your admin account, optionally enable 2FA.

## Develop

Two terminals:

```bash
make dev-api    # Go backend with hot reload (requires `air`)
make dev-web    # Vite dev server
```

Open <http://localhost:5173>. Vite proxies `/api` and `/ws` to the Go backend
on `:8443` (`CR_DEV` mode → plain HTTP, non-Secure cookies).

Prerequisites: **Go 1.25+**, **Node 20+**, [`air`](https://github.com/air-verse/air)
for backend hot reload.

## Project layout

```
cmd/controlroom/                  # entrypoint
internal/
  api/                            # HTTP routes + middleware
    {auth,containers,k8s,logs,network,services,settings,setup,system,terminal,updates}/
  auth/                           # password, TOTP, JWT, sessions, ratelimit
  cert/                           # self-signed + ACME (autocert)
  collectors/                     # /proc and /sys readers
  config/                         # env-based config
  docker/                         # Docker client wrapper
  jobs/                           # generic job runner with ring buffer
  k8s/                            # client-go wrapper (read-only)
  logs/                           # journalctl wrapper
  network/                        # ip + ufw wrappers
  pty/                            # PTY session manager (incl. host-shell + login flow)
  store/                          # SQLite migrations + accessors
  systemd/                        # dbus client wrapper
  web/                            # embed.FS for the SPA
web/                              # Vite + React + TS + Tailwind + shadcn/ui
deploy/
  Dockerfile                      # fat-privileged container image
  docker-compose.yml              # docker deployment
  install.sh                      # bare-metal installer
  controlroom.service             # hardened systemd unit
  controlroom.sudoers             # tight sudoers fragment
  k8s/                            # in-cluster manifests
  scripts/                        # host-side helpers (e.g. setup-host-kubeconfig.sh)
docs/
  INSTALL.md                      # per-shape install + first-run
  CONFIG.md                       # env vars + per-tab requirements
  SECURITY.md                     # threat model per deployment shape
```

## Documentation

- [`docs/INSTALL.md`](./docs/INSTALL.md) — detailed install for all three shapes.
- [`docs/CONFIG.md`](./docs/CONFIG.md) — every environment variable + which tabs need what.
- [`docs/SECURITY.md`](./docs/SECURITY.md) — threat model and what's actually enforced.
- [`CHANGELOG.md`](./CHANGELOG.md) — release-by-release feature + fix log.
- [`SPEC.md`](./SPEC.md) — design spec.
- [`MILESTONES.md`](./MILESTONES.md) — what shipped when.

## License

MIT — see [`LICENSE`](./LICENSE).
