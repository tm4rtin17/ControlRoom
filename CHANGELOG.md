# Changelog

All notable changes to ControlRoom are documented here. The format is loosely
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project follows [Semantic Versioning](https://semver.org/) — minor releases
(0.x.0) deliver feature work and may include backward-incompatible changes
to deploy artifacts before 1.0.

## [Unreleased]

Tracks ongoing work on `main` after the v0.2 release.

## [0.2.0] — 2026-05-09

The Kubernetes release. Adds a full-feature Kubernetes management tab,
turns the Docker container into an option that drives every host
integration, fixes CI from end to end, and tightens up several rough
edges discovered along the way.

### Added — Kubernetes tab

- **Phase A — read-only inventory**: list views for Nodes, Namespaces,
  Workloads (Deployment / StatefulSet / DaemonSet), Pods, Services. Backed
  by `client-go`. New `internal/k8s/` package with auto-detection of
  in-cluster auth, `$KUBECONFIG`, `~/.kube/config`, and `/etc/rancher/k3s/k3s.yaml`
  in that order. Capability-gated: `/api/system/capabilities` reports
  `kubernetes: bool` so the SPA hides the tab when no cluster is reachable.
  ([#9e2d3fe])
- **Phase A — in-cluster deployment manifests** at `deploy/k8s/`:
  Namespace + ServiceAccount + ClusterRole + ClusterRoleBinding +
  Deployment + Service + Ingress + PVC. Pod runs as nonroot uid 65532,
  drop ALL caps, readOnlyRootFilesystem. ([#9e2d3fe])
- **Phase B — detail drawers + pod log streaming**: per-resource Sheet
  drawers showing conditions, events, container statuses, etc. Live pod
  log streaming via WebSocket, multi-container picker, pause/resume.
  ([#dff99cc])
- **Phase C — lifecycle actions**: restart workload (annotation patch),
  scale Deployment / StatefulSet (UpdateScale subresource preserves
  resourceVersion for optimistic concurrency), delete pod (controller-
  recreate), cordon / uncordon node. Every action writes an audit row
  with target + intent. ([#7755b9a])
- **Phase D1 — pod exec**: SPDY → WebSocket bridge so the existing xterm
  in the SPA can attach to any container. Frame protocol mirrors
  `/ws/terminal` so the frontend cloned `Terminal.tsx` with minimal
  changes. Container picker, command picker (`/bin/sh`/`/bin/bash`/
  custom), audit on session start + end. ([#4ffa357])
- **Phase D2 — ConfigMap viewer + editor**: list view; structured
  key/value editor (validates DNS-1123 keys, enforces 1 MiB cap). Update
  via PUT (not Patch) so resourceVersion conflicts surface as 409 with a
  dedicated Reload UX instead of silent overwrite. ([#33d3b0c])
- **Phase D3 — Secret viewer (read-only)**: list + masked detail viewer.
  Eight fixed bullets regardless of value length so the DOM never leaks
  size. Per-row reveal-on-click + copy-to-clipboard with insecure-context
  fallback. JSON pretty-print and PEM detection. Every detail GET writes
  a `k8s.secret.read` audit row carrying `key_count` only — never names
  or values. RBAC widening is `secrets: get,list`; `watch` is
  intentionally excluded so we don't keep a long-lived stream of all
  cluster secret values open. ([#8bb8564])
- **Phase D4 — Monaco YAML editor**: edit any of Deployment / StatefulSet /
  DaemonSet / Service / ConfigMap as YAML. `metadata.{managedFields,
  creationTimestamp,uid,resourceVersion,generation}` and the entire
  `status` are stripped from the editable buffer; resourceVersion is sent
  separately so optimistic concurrency works. Dry-run via
  `metav1.DryRunAll` validates with admission webhooks before commit.
  Apply requires AlertDialog confirmation. Monaco lazy-loaded via
  `React.lazy` so the main bundle stays the same size. URL/body
  cross-check (`kind` / `metadata.namespace` / `metadata.name` in YAML
  must match URL params) prevents cross-resource attacks. ([#72b454d])

### Added — non-Kubernetes

- **Capability-gated navigation** (`/api/system/capabilities`): the SPA
  hides Services, Containers, Kubernetes (and others) when their backend
  isn't reachable instead of showing dead-end 503 pages. ([#395b640])
- **Logs page falls back to Docker container logs** when journald isn't
  available, with a Source toggle so the operator can view either.
  ([#638fbdb])
- **Tailscale CGNAT awareness** in the public-bind warning banner —
  100.64.0.0/10 (RFC 6598) is treated as private. ([#638fbdb])
- **Fat-privileged Docker deployment**: optional. The `deploy/docker-compose.yml`
  flavor now runs as root with `network_mode: host`, `pid: host`,
  `cap_add: SYS_ADMIN/SYS_PTRACE/NET_ADMIN`, and host bind mounts so
  every host integration (Services, Updates, Network, Logs/journal,
  Terminal, Kubernetes) works from a container. Image switches from
  distroless to `debian:bookworm-slim` and grows from ~25 MB to ~230 MB.
  Documented as the higher-blast-radius option; bare-metal install
  remains the unprivileged path. ([#f52c126])
- **Terminal — PAM-authenticated host login**: when `CR_HOST_SHELL=true`
  and `CR_TERMINAL_LOGIN=true` (set by the fat-privileged compose), the
  terminal spawns a host shell through `nsenter -t 1`. Operator types
  username, then the spawned bash drops to nobody via `setpriv` and
  `exec su -l <user>` runs full PAM authentication (lockouts and
  `/var/log/auth.log` entries flow through normally). Without the
  privilege drop, root invoking `su` would skip authentication. ([#2d353e7],
  [#f0ca1f7], [#aca5187])
- **`docs/CONFIG.md`**: new reference covering every `CR_*` env var, a
  per-tab capability matrix across the three deployment shapes, per-shape
  data paths, and capability-detection debugging. ([#f58ccf3])

### Changed

- **README, INSTALL, SECURITY** rewritten for the three deployment shapes
  (bare-metal / fat-privileged container / in-cluster Pod) with explicit
  compromise-impact statements per shape. ([#f58ccf3])
- **`deploy/docker-compose.yml`** now declares `controlroom-data` as
  `external: true` so `docker compose up` and raw `docker run -v
  controlroom-data:/data` always land on the same persistent volume.
  Without this, switching between the two silently created
  `deploy_controlroom-data` and the operator's admin account vanished
  on the next boot. First-time install now requires
  `docker volume create controlroom-data`. ([#a4187c8])

### Fixed

- **Container-deployment Docker tab returned `null` for ports/labels**,
  blanking the SPA. Initialize Docker `Container` slice/map fields to
  non-nil empty values before JSON encoding. ([#74dd538])
- **Network tab blank-screen** with the same nil-slice JSON
  serialization bug. `Interface.IPs`, `Interface.Flags`, and
  `UFWStatus.Rules` now default to `[]` not `null`. ([#83deeaa])
- **Logs page returned 500** when running in the distroless container
  (no `journalctl`). Now returns a clean 503 with a "switch to
  Containers source or run on bare metal" message; the SPA shows the
  real error instead of "Could not fetch logs." ([#638fbdb])
- **Services / Containers tabs were dead-ends** when the host integration
  was missing. Capability-gated nav now hides them. ([#395b640])

### CI

- The CI pipeline had been failing at workflow startup since v0.1 (the
  `m9 polish` push to main also failed at 0s). Fixed in five steps:
  - Pin Go version from `go.mod` instead of a hard-coded `1.23` that
    drifted out of sync. ([#f7917ac])
  - Block-scalar the awk format string in the image-size step. The
    string `"image size: %.1f MB\n"` made YAML's plain-scalar parser
    treat `image size:` as a key. ([#8ad0356])
  - Track `web/package-lock.json` so `npm ci` works and the CI cache
    step resolves; bump golangci-lint action `@v6` → `@v8` and pin to
    v2.5.0 (built with Go 1.25). ([#675dbb9])
  - Add a minimal `.golangci.yml` (v2 doesn't run anything without one);
    fix the two real findings (redundant assignment in `internal/docker/fake.go`,
    unused stub type in `internal/api/auth/auth.go`). ([#0673ce8])
  - Align Dockerfile COPY path with vite's `outDir` —
    `/src/web/dist` not `/src/internal/web/dist`. ([#c8fb01c])
- Added `go vet ./...` as a separate step (faster feedback than running
  the full test suite). Scoped `go test` to `./internal/... ./cmd/...`
  so it doesn't descend into `web/node_modules/flatted/golang/pkg/flatted`
  (an npm dep that ships an embedded `.go` file). ([#f7917ac])

### Security

- **Per-Secret-read audit** (`k8s.secret.read`): every detail GET, success
  or failure, writes a row carrying `key_count` only. Recoverable
  read-history without leaking what was read.
- **Per-manifest-edit audit** (`k8s.manifest.apply` / `…dry_run`):
  records `kind`, `namespace`, `name`, `bytes` of the YAML payload —
  never the body itself.
- **Secret value mass-leakage**: 8 fixed-length bullets in the SPA so
  the DOM doesn't expose value length even when masked.
- **Cross-resource attack on manifest edit**: server enforces that the
  YAML's `kind` / `metadata.namespace` / `metadata.name` match the URL
  params. ([#72b454d])

### Migration notes

- **Container deployment**: run `docker volume create controlroom-data`
  once before `docker compose up`. The compose volume is now `external`.
  If you upgraded mid-session and your data appears empty, the data is
  most likely on `deploy_controlroom-data` (project-namespaced volume
  created by the prior compose; the original is still on
  `controlroom-data`).
- **In-cluster deployment**: re-apply `deploy/k8s/rbac.yaml` to pick up
  the widened verbs (`pods/exec: create`, `pods: delete`,
  `nodes: patch`, `apps/*: patch,update`, `apps/*/scale: update`,
  `services/configmaps: update`, `secrets: get,list`).

[#9e2d3fe]: https://github.com/tm4rtin17/ControlRoom/commit/9e2d3fe
[#dff99cc]: https://github.com/tm4rtin17/ControlRoom/commit/dff99cc
[#7755b9a]: https://github.com/tm4rtin17/ControlRoom/commit/7755b9a
[#4ffa357]: https://github.com/tm4rtin17/ControlRoom/commit/4ffa357
[#33d3b0c]: https://github.com/tm4rtin17/ControlRoom/commit/33d3b0c
[#8bb8564]: https://github.com/tm4rtin17/ControlRoom/commit/8bb8564
[#72b454d]: https://github.com/tm4rtin17/ControlRoom/commit/72b454d
[#395b640]: https://github.com/tm4rtin17/ControlRoom/commit/395b640
[#638fbdb]: https://github.com/tm4rtin17/ControlRoom/commit/638fbdb
[#f52c126]: https://github.com/tm4rtin17/ControlRoom/commit/f52c126
[#2d353e7]: https://github.com/tm4rtin17/ControlRoom/commit/2d353e7
[#f0ca1f7]: https://github.com/tm4rtin17/ControlRoom/commit/f0ca1f7
[#aca5187]: https://github.com/tm4rtin17/ControlRoom/commit/aca5187
[#f58ccf3]: https://github.com/tm4rtin17/ControlRoom/commit/f58ccf3
[#a4187c8]: https://github.com/tm4rtin17/ControlRoom/commit/a4187c8
[#74dd538]: https://github.com/tm4rtin17/ControlRoom/commit/74dd538
[#83deeaa]: https://github.com/tm4rtin17/ControlRoom/commit/83deeaa
[#f7917ac]: https://github.com/tm4rtin17/ControlRoom/commit/f7917ac
[#8ad0356]: https://github.com/tm4rtin17/ControlRoom/commit/8ad0356
[#675dbb9]: https://github.com/tm4rtin17/ControlRoom/commit/675dbb9
[#0673ce8]: https://github.com/tm4rtin17/ControlRoom/commit/0673ce8
[#c8fb01c]: https://github.com/tm4rtin17/ControlRoom/commit/c8fb01c

## [0.1.0] — 2026-05-08

Initial release. See [`MILESTONES.md`](./MILESTONES.md) for the full M1–M9
breakdown.

Highlights:

- Single Go binary serving an embedded Vite/React SPA over HTTPS.
- Tabs: Dashboard, Updates, Services, Containers, Terminal, Network,
  Logs, Settings.
- Auth: bcrypt(cost 12) + optional TOTP, HS256 access JWT (15 min) +
  rotating opaque refresh tokens (7 days) with reuse detection,
  per-IP rate limit + per-user backoff.
- TLS modes: self-signed, Let's Encrypt (HTTP-01 via autocert), and
  proxy.
- Bare-metal installer (`deploy/install.sh`): hardened systemd unit,
  scoped sudoers fragment, dedicated `controlroom` system user.
- Docker Compose deployment as a (then-distroless) alternative.
- Audit log for every privileged action; keystrokes never recorded.

[Unreleased]: https://github.com/tm4rtin17/ControlRoom/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/tm4rtin17/ControlRoom/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tm4rtin17/ControlRoom/releases/tag/v0.1.0
