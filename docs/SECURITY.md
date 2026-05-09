# Security model

This document describes ControlRoom's threat model and the implementation
choices made through v0.2. It tracks what's *actually shipped*; for the
forward-looking design, see [`../SPEC.md`](../SPEC.md) §5.

## Threat model

ControlRoom is designed for a single trusted operator (or a small team) on
a LAN, with optional remote access via a VPN (Tailscale / WireGuard / SSH
tunnel) or a TLS-terminating reverse proxy with strict firewall rules.
**It is not designed to be exposed raw to the public internet.**

In scope:
- Network attacker who can read TLS-protected requests but can't break
  TLS 1.2+ ciphers.
- Stolen credentials (password leak; single device compromise).
- Stolen refresh tokens (cookie exfiltration).
- Cross-site request forgery from another origin.
- Brute force of login.

Out of scope:
- Host compromise. Root on the box bypasses every mitigation here.
- Side-channel attacks against the JWT signing key on disk.
- Authenticator-app compromise (TOTP is a second factor, not a panacea).
- Supply-chain compromise of upstream dependencies.

## Authentication & sessions

| Surface | Implementation |
|---|---|
| Password hashing | bcrypt cost 12 (`internal/auth/password.go`). |
| TOTP | RFC 6238, 6 digits / 30 s / SHA-1, ±1-step verify (`internal/auth/totp.go`). Optional but recommended. |
| Access token | HS256 JWT, 15 min TTL. Key in `$CR_DATA_DIR/jwt.key` (mode 0600). |
| Refresh token | Opaque `<session_id>.<32-byte-secret>`; only sha256 stored in `sessions.refresh_hash`. 7 day TTL by default (`CR_SESSION_HOURS`). |
| Rotation | Every refresh creates a new session in the same `family_id` and revokes the parent. |
| Reuse detection | Presenting a revoked session that already has a child → entire family revoked (`internal/auth/sessions.go`). Logged-out childless sessions are simply invalid (no family burn). |
| Brute force | Per-IP token bucket (5/min) on login + per-username exponential backoff (30 s → 1 h cap). |

## Cookies

| Cookie | Path | Flags | Purpose |
|---|---|---|---|
| `cr_access` | `/` | HttpOnly, Secure, SameSite=Strict | Access JWT (sent on `/api` and `/ws`). |
| `cr_refresh` | `/api/auth` | HttpOnly, Secure, SameSite=Strict | Opaque rotating refresh token. |
| `cr_csrf` | `/` | Secure, SameSite=Strict, **not** HttpOnly | Double-submit token; SPA mirrors it in `X-CSRF-Token`. |
| `cr_setup` | `/api/setup` | HttpOnly, Secure, SameSite=Strict | Short-lived (15 min) setup-wizard JWT. |

`Secure` is dropped only when `CR_DEV=true` so plain-HTTP development works.

## CSRF

Double-submit pattern. The CSRF middleware is mounted on the **authenticated**
group only — login/refresh/setup are exempt because they have no prior session
to mirror from. `SameSite=Strict` on every cookie blocks cross-origin POSTs in
modern browsers.

## Privilege scoping by deployment shape

The privilege model differs significantly between the three deployment
shapes — pick the one whose blast radius matches your threat model.

### Shape A — Bare-metal (`deploy/install.sh`)

The default path. Lowest privilege.

The systemd unit (`deploy/controlroom.service`) runs as a dedicated
`controlroom` system user with:

- `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`,
  `PrivateTmp`, `PrivateDevices`, kernel-tunables/modules/cgroups protections.
- `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK`.
- `SystemCallFilter=@system-service & ~@privileged @resources`.
- `SupplementaryGroups=adm docker` so it can read the journal and talk to the
  Docker socket when present.

A tight `/etc/sudoers.d/controlroom` fragment NOPASSWDs only the exact
commands ControlRoom needs:
- `apt-get update`, `apt-get install -y --only-upgrade …`
- `systemctl reboot`
- The specific `ufw` verbs used by `/api/network/firewall/*`

Everything else still requires the operator to authenticate. The fragment
is validated with `visudo -c` before installation — `install.sh` aborts
on parse failure.

**Compromise impact**: an attacker who pwns the controlroom binary can:
- Read the journal (group `adm`).
- Talk to the Docker daemon (group `docker`).
- Run the exact sudoers-allowlisted commands as root.
- Read `$CR_DATA_DIR` (the JWT key, the audit log, the SQLite DB).

They **cannot** spawn arbitrary root shells, edit `/etc/sudoers`, install
packages outside the allowlisted apt verbs, or read other users' homes.

### Shape B — Docker container (`deploy/docker-compose.yml`)

The fat-privileged path. Highest blast radius. Read this section.

The container runs as **root** (uid 0) with:
- `network_mode: host` — shares the host's network namespace.
- `pid: host` — shares the host's PID namespace; `nsenter -t 1` reaches
  systemd as PID 1.
- `cap_add: [SYS_ADMIN, SYS_PTRACE, NET_ADMIN]`.
- `security_opt: [apparmor:unconfined, seccomp:unconfined]`.

And bind-mounts:
- `/var/run/docker.sock:ro` (the Docker daemon)
- `/var/run/dbus/system_bus_socket:rw` (host systemd dbus)
- `/run/log/journal`, `/var/log/journal`, `/etc/machine-id` (host journal)
- `/etc/rancher/k3s/k3s.yaml:ro` (kubeconfig with cluster-admin creds)
- `/var/cache/apt:rw`, `/etc/apt:ro`, `/var/lib/dpkg:ro` (host apt state)

**Compromise impact**: an attacker who pwns the controlroom binary in this
shape has **effective root on the host**. They can `nsenter -t 1` to PID 1
and execute arbitrary commands as root, read the K3s admin kubeconfig and
take over the cluster, write to `/var/cache/apt` and arrange for any apt
operation to install attacker-controlled packages, talk to the Docker
daemon and create privileged containers.

This is the deliberate tradeoff for getting all host integrations working
from a container without sidecars. **Use only on single-user homelabs
where the operator already has root** and where the convenience is
explicitly worth the loss of container isolation.

If your threat model doesn't accept this, use Shape A.

### Shape C — In-cluster Kubernetes Pod (`deploy/k8s/`)

Cluster-only path. Constrained privilege within the cluster, no host
reach.

The Pod runs as:
- `runAsNonRoot: true`, `runAsUser: 65532`, `runAsGroup: 65532`,
  `fsGroup: 65532` (so the PVC is writable by the nonroot user).
- `seccompProfile: RuntimeDefault`.
- `allowPrivilegeEscalation: false`.
- `readOnlyRootFilesystem: true` (binary doesn't need a writable root;
  `/data` is the only writable mount, via the PVC).
- `capabilities: { drop: [ALL] }`.

Cluster permissions are scoped via the `controlroom-reader` `ClusterRole`.
The role grew in tightly-scoped steps as Phases A → D landed; the current
shape is in `deploy/k8s/rbac.yaml`. Headline:

| Resource | Verbs | Why |
|---|---|---|
| `nodes`/`namespaces`/`pods`/`services`/`configmaps`/`events`/`endpoints` | `get,list,watch` | Read views (Phase A) |
| `apps/deployments`/`statefulsets`/`daemonsets`/`replicasets` | `get,list,watch` | Workload reads (A) |
| `pods/log` | `get` | Pod log streaming (B) |
| `pods/exec` | `create` | Browser exec (D1) |
| `pods` | `delete` | Pod rotate (C) |
| `nodes` | `patch` | Cordon/uncordon (C) |
| `apps/deployments`/`statefulsets`/`daemonsets` | `patch,update` | Restart annotation + YAML edit (C + D4) |
| `apps/deployments/scale`/`statefulsets/scale` | `update` | Scale (C) |
| `services`/`configmaps` | `update` | Edit (D2 + D4) |
| `secrets` | `get,list` only | Read-only viewer (D3) |

**Explicitly excluded** at every phase boundary:
- `secrets/watch` — would keep a long-lived stream of every cluster
  secret value open. Point-in-time reads + per-read audit (see below)
  are the trade.
- `delete` on `apps/*` — would let the SPA wipe a Deployment / STS / DS
  outside whatever GitOps tooling owns it.
- `persistentvolumes` (cluster-scoped) and `persistentvolumeclaims`.
- `create` on most resources — Phase D4's manifest editor is
  edit-existing-only; there's no create-from-nothing path.

**Per-read audit** for sensitive resources:
- `k8s.secret.read` on every Secret detail GET (success or failure),
  detail = `{"key_count": N}` only — never key names or values.
- `k8s.manifest.apply` / `k8s.manifest.dry_run` on every YAML edit, with
  `bytes` of the edited YAML but never the body itself.

**Compromise impact**: an attacker who pwns the controlroom binary in
this shape can read everything the ClusterRole grants (cluster inventory,
pod logs, configmaps, secret values), and can mutate the limited write
surface (rollout restart, scale, delete pod, cordon, configmap update,
manifest YAML update on the 5 editable kinds). They **cannot** create
new resources, delete workloads, watch secrets, escape the Pod, or
reach the host. The audit log records who did what to which target.

This is still the lowest-risk shape vs. the fat-privileged container,
just no longer "read-only" after Phase D.

## Audit

Every privileged action writes a row to `audit_log` (best-effort; never
fails the parent request):

- Auth: login success/failure (with reason), logout, refresh, refresh-reuse,
  TOTP enable/disable, password change.
- Setup: token verify, complete (with `totp_enabled`).
- Services / containers: each lifecycle action with target + outcome.
- Updates: check / apply / reboot job starts.
- Firewall: rule add/delete + enable/disable with the rule spec.
- Terminal: `session_start` and `session_end` (duration, bytes_in,
  bytes_out, exit code). **Keystrokes are never recorded.**

For Shape B specifically, host-level auth events flow through PAM rather
than ControlRoom's audit table:
- The Terminal's `su -l <user>` invocation writes to `/var/log/auth.log`
  via `pam_unix`. Lockouts (`pam_faillock` if configured) apply.
- `apt-get` operations are logged in `/var/log/apt/history.log` and
  `/var/log/apt/term.log`.

This is intentional — the host's audit trail survives ControlRoom
restarts and re-creates.

Retention is unbounded today — see "Known gaps" below.

## TLS

Three modes, set with `CR_TLS_MODE`. See [`INSTALL.md` → TLS modes](./INSTALL.md#tls-modes)
for operator instructions.

In every mode:
- TLS 1.2+ only. Older protocols disabled.
- Cipher suites exclude CBC / RC4 / 3DES; TLS 1.3 uses the std-lib's
  fixed list.
- ALPN advertises `http/1.1` only (no h2 in this version — h2 had a
  goroutine leak interaction with the WS upgrader that's still being
  triaged).
- ECDSA P-256 keys preferred over RSA where we generate.

## Public-bind detection

The SPA shows a destructive banner ("Public-looking address") when reached
from an IP that doesn't look like:
- RFC 1918 (10/8, 172.16/12, 192.168/16)
- Link-local (169.254/16, fe80::/10)
- IPv6 ULA (fc00::/7)
- Loopback (127/8, ::1)
- **RFC 6598 / Tailscale CGNAT (100.64.0.0/10)** — added so admins reaching
  ControlRoom over Tailscale don't get a misleading warning.

This is a heuristic on `window.location.hostname` — it's about the URL the
operator dialed, not what the server is bound to. False negatives are
possible (e.g. a bare hostname like `homelab.local`).

## Known gaps (post-v0.2)

These are documented and tracked, not silently missing:

- **Settings persistence**: host display name and the version-check toggle
  are read from env only — no UI write yet. Promotion to a settings table
  comes with the v0.3 RBAC work.
- **Audit log retention/rotation**: grows unbounded today.
- **HSTS / CSP headers**: planned for the next polish pass.
- **Netplan editing**: read-only interfaces today.
- **TLS-via-cert-manager** in the in-cluster shape: the default
  `deploy/k8s/ingress.yaml` leaves TLS as a TODO comment for the
  operator's chosen ClusterIssuer.
- **RBAC user roles**: `users.role` is stored but not enforced — every
  authenticated user has admin authority across all tabs. Real
  multi-user RBAC is a v0.3 item.
- **Multi-cluster Kubernetes**: the K8s tab targets exactly one cluster
  (whatever the in-cluster SA reaches or the kubeconfig's first context
  points at). No context switcher.

Closed in v0.2 (these were gaps in v0.1):
- **K8s tab**: Phases A–D shipped — read-only inventory, detail drawers,
  pod log streaming, pod exec, lifecycle actions (restart / scale / delete /
  cordon), ConfigMap and Secret viewers, Monaco YAML editor.
- **Public-bind detection**: now treats Tailscale CGNAT (100.64.0.0/10)
  as private so admins reaching ControlRoom over Tailscale don't see a
  misleading warning.

## Reporting

Pre-1.0: open a GitHub issue with the `security` label, or email the
maintainer if you need to disclose privately.
