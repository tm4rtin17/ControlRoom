# ControlRoom — Project Specification

> Lightweight, modern web UI for managing headless Linux homelab servers.
> Replaces routine SSH for system overview, package updates, services, containers, networking, and logs — without becoming a heavyweight platform.

**Status:** Draft v0.1 — 2026-05-08
**Target footprint:** ≤50 MB RAM idle, ≤25 MB binary, single static image.

---

## 1. Goals & Non-Goals

### Goals
- One binary, one port, one config file. Drop on a fresh Debian/Ubuntu host and it works.
- Mobile-first, dark-mode-first UI that feels calm and premium (shadcn/ui aesthetic).
- HTTPS by default; safe defaults that fail closed.
- Manage both systemd services and Docker/Podman containers in one place.
- Real-time: live logs, live stats, live terminal — all over WebSocket.

### Non-Goals (explicitly out of scope)
- Multi-host fleet management (this is single-host; pair with a reverse proxy if you have several).
- Kubernetes / cluster orchestration (use Headlamp / Lens / k9s).
- Configuration management (use Ansible / NixOS).
- Public SaaS / multi-tenant. Single-org, small user list (≤10).
- Non-Debian/Ubuntu distros at v0.1. (Fedora/Arch deferred — `apt`-specific code lives behind a `pkg` interface so a `dnf`/`pacman` backend can drop in later.)

---

## 2. Architecture

```
                                    ┌──────────────────────────────────┐
                                    │       Browser (mobile/desktop)   │
                                    │  React SPA · TanStack Query · WS │
                                    └─────────────┬────────────────────┘
                                                  │ HTTPS / WSS
                                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      controlroom (single Go binary)                 │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────┐   │
│  │  embed.FS    │  │  REST /api   │  │  WebSocket /ws          │   │
│  │  static SPA  │  │  Fiber v3    │  │  terminal · logs · stats│   │
│  └──────────────┘  └──────┬───────┘  └─────────┬───────────────┘   │
│                           │                    │                    │
│              ┌────────────┴────────────┬───────┴──────────┐         │
│              ▼                         ▼                  ▼         │
│      ┌──────────────┐         ┌────────────────┐   ┌────────────┐  │
│      │ auth · audit │         │ collectors     │   │ pty mgr    │  │
│      │ store (SQLite│         │ /proc /sys     │   │ creack/pty │  │
│      │ +bcrypt+TOTP)│         │ dbus  docker   │   │            │  │
│      └──────────────┘         │ netlink ufw    │   └────────────┘  │
│                               └────────────────┘                    │
└─────────────────────┬───────────────────────────────────────────────┘
                      │
                      ▼
              Host kernel / dbus / docker.sock / journald
```

### Why single-binary (no separate agent)
A separate "agent" makes sense for *remote* targets — but ControlRoom runs *on* the box it manages. Adding an agent would just be IPC overhead. Privilege scoping is handled via Linux capabilities + a tight sudoers fragment, not process separation.

---

## 3. Final Stack

| Layer        | Choice                                | Why                                                               |
|--------------|---------------------------------------|-------------------------------------------------------------------|
| Backend      | **Go 1.23 + Fiber v3**                | Tiny static binary, mature stdlib, best-in-class systemd/docker libs, fast iteration. |
| HTTP server  | Fiber v3 (fasthttp)                   | Lower mem than net/http for many small connections; idiomatic middleware. |
| WebSocket    | `gofiber/contrib/websocket`           | Same router as REST; no separate gorilla setup.                   |
| Terminal     | `creack/pty` + WS                     | True PTY, no `ttyd` dependency, works in scratch image.           |
| systemd      | `coreos/go-systemd/v22/dbus`          | First-party-quality dbus binding; handles units cleanly.          |
| Docker       | `docker/docker/client`                | Official; talks to `/var/run/docker.sock`.                        |
| Storage      | SQLite via `modernc.org/sqlite`       | Pure-Go (no CGO), single file, perfect for users/sessions/audit.  |
| Auth         | JWT (HS256) in httpOnly cookie + bcrypt(cost 12) + TOTP (`pquerna/otp`) | Standard, audited primitives; no external IdP required.           |
| Logging      | `rs/zerolog`                          | Structured, low-alloc, JSON-out for journald.                     |
| Config       | env + flag (no viper)                 | Keep deps small; 12-factor friendly.                              |
| Frontend     | **Vite 5 + React 18 + TS**            | Fast HMR, small bundle, no Node runtime in production.            |
| UI kit       | Tailwind CSS + shadcn/ui              | Premium feel, owned components (no runtime lib bloat).            |
| Data fetch   | TanStack Query v5                     | Cache, retries, polling, WS-friendly invalidation.                |
| Local state  | Zustand                               | 1 KB; no Redux ceremony.                                          |
| Forms        | react-hook-form + zod                 | Tiny, type-safe.                                                  |
| Terminal UI  | xterm.js                              | De-facto standard; 200 KB gzip.                                   |
| Charts       | Recharts (overview only)              | One small chart lib; lazy-loaded.                                 |
| Build        | Makefile + multi-stage Docker         | `make build` → static binary; `make image` → ~20 MB scratch image.|

**Rejected:** Next.js (Node runtime busts the memory target for an authenticated dashboard with no SEO need), Rust/Axum (slower iteration; less mature sysadmin libs), ttyd (extra binary).

---

## 4. Folder Structure

```
controlroom/
├── cmd/controlroom/main.go         # entrypoint: flags, signals, wiring
├── internal/
│   ├── api/                        # HTTP handlers, grouped by domain
│   │   ├── router.go               # mounts all routes + middleware
│   │   ├── middleware/             # auth, csrf, ratelimit, audit, recover
│   │   ├── auth/                   # login, logout, refresh, 2fa
│   │   ├── system/                 # overview, cpu, mem, disks, temps, power
│   │   ├── updates/                # apt updates check & apply
│   │   ├── services/               # systemd units
│   │   ├── containers/             # docker/podman
│   │   ├── network/                # interfaces, firewall (ufw)
│   │   ├── logs/                   # journalctl + container logs
│   │   ├── terminal/               # PTY over WS
│   │   └── settings/               # users, prefs
│   ├── auth/                       # JWT, password, TOTP, sessions
│   ├── store/                      # SQLite: users, sessions, audit
│   ├── collectors/                 # /proc, /sys, sensors readers
│   ├── systemd/                    # dbus wrapper (testable interface)
│   ├── docker/                     # docker client wrapper
│   ├── network/                    # netlink + ufw wrappers
│   ├── pty/                        # session manager (size, resize, EOF)
│   ├── audit/                      # append-only structured log
│   └── config/                     # env + flag parsing, defaults
├── web/                            # Vite SPA (built into embed.FS)
│   ├── src/
│   │   ├── routes/                 # /, /services, /containers, /terminal, …
│   │   ├── components/ui/          # shadcn primitives (owned)
│   │   ├── components/             # app-specific composites
│   │   ├── lib/api.ts              # typed fetch + WS client
│   │   ├── lib/auth.ts             # session hooks
│   │   ├── stores/                 # zustand stores
│   │   └── main.tsx
│   ├── index.html
│   ├── tailwind.config.ts
│   ├── vite.config.ts
│   └── package.json
├── deploy/
│   ├── Dockerfile                  # multi-stage: node build → go build → scratch
│   ├── docker-compose.yml          # one-service compose with sane defaults
│   ├── controlroom.service         # systemd unit for bare-metal install
│   ├── controlroom.sudoers         # tight sudoers fragment for privileged ops
│   └── install.sh                  # one-command bare-metal installer
├── scripts/
│   └── dev.sh                      # vite dev + go run with hot reload
├── docs/
│   ├── SECURITY.md
│   ├── INSTALL.md
│   └── API.md                      # auto-generated from handler annotations
├── go.mod / go.sum
├── Makefile
├── SPEC.md                         # this file
└── README.md
```

---

## 5. Security Model

Threat model: a single trusted operator (or small team) on a LAN; occasional remote access via reverse proxy / VPN. **Not** designed to be exposed raw to the public internet — we'll warn loudly in the UI if we detect a public bind.

### Transport
- HTTPS by default. Three modes, picked at install time:
  1. **Self-signed** (LAN default) — generated on first boot, 10-year cert, stored in `$DATA_DIR/tls/`.
  2. **Let's Encrypt autocert** — `--acme-host=ctrl.example.com --acme-email=…`. HTTP-01 on :80.
  3. **Behind reverse proxy** — `--trust-proxy` enables `X-Forwarded-*`; ControlRoom binds plain HTTP on loopback only.
- HSTS on by default once non-self-signed cert detected.
- TLS 1.2 minimum; modern cipher suites only.

### Authentication
- Username + bcrypt(cost=12) password.
- Optional TOTP (RFC 6238) — required for accounts with `admin` role if enabled in settings.
- JWT access token (15 min) + refresh token (7 days), both in `httpOnly; Secure; SameSite=Strict` cookies.
- Refresh tokens are rotating + family-tracked: reuse triggers full family revocation (detects token theft).
- Brute force: per-IP rate limit on `/api/auth/login` (5/min) + per-username exponential backoff (30s → 1h cap).
- **First-run setup wizard:** on first boot with an empty users table, the server generates a one-time setup token and prints it once to stdout + journal. The browser is redirected to `/setup`; the operator pastes the token, then completes a short wizard (username → password with strength meter → optional TOTP enrollment). The token check prevents a network scanner from claiming admin before the operator does. Once the admin exists, `/setup` is permanently 410 Gone.

### Authorization
- **v0.1 ships single-user.** Exactly one admin account exists; user-management endpoints are absent. All authenticated routes require the admin session; no operator/viewer split yet.
- The `users.role` column ships in the v0.1 schema (default `'admin'`) so v0.2 can enable RBAC without a migration.
- v0.2 roles: `admin` (everything), `operator` (no user mgmt, no firewall edits, no shutdown), `viewer` (read-only). Every privileged handler will declare its required role via middleware then.
- Throughout this spec, the **Role** columns describe v0.2 RBAC; in v0.1 every listed route requires only "authenticated."

### CSRF / XSS
- Cookie auth → CSRF risk → double-submit token: server sets non-httpOnly `csrf` cookie, client mirrors it in `X-CSRF-Token` header for state-changing methods. Constant-time compare.
- CSP: `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data:; frame-ancestors 'none'`. (Tailwind needs inline styles; everything else hashed.)
- No inline event handlers in the SPA.

### Privilege scoping
- Binary runs as a dedicated `controlroom` user, **not root**.
- Capabilities granted via systemd unit: `CAP_NET_ADMIN` (firewall), `CAP_SYS_TIME` (read-only chronyc), no `CAP_SYS_ADMIN`.
- A tight `/etc/sudoers.d/controlroom` allows specific commands without password: `apt-get update`, `apt-get -y upgrade`, `systemctl <verb> <unit>`, `ufw …`, `shutdown -h now`, `reboot`. Each command is logged via sudo + ControlRoom's own audit log.
- Docker access via group membership in `docker` (documented as a privilege escalation risk in `SECURITY.md`).

### Audit
- Every privileged action writes a structured row to `audit.db`: `(ts, user_id, ip, method, path, target, outcome, detail)`.
- Append-only; UI shows last 30 days; older entries rotated to `$DATA_DIR/audit-YYYY-MM.jsonl`.

### Misc hardening
- All inputs validated with zod (frontend) + a small validator pkg (backend). Service/container names regex-locked.
- No string interpolation into shell commands — `exec.Command(name, args...)` only.
- Request body size cap (1 MB), WS frame cap (64 KB).
- Public-bind warning banner when `0.0.0.0` + non-LAN client IP detected.

---

## 6. API Surface (Phase 1)

> All routes under `/api`. Auth required except `POST /api/auth/login`, `POST /api/auth/setup`, `GET /api/healthz`. WebSocket routes under `/ws`. CSRF token required on POST/PATCH/DELETE.

### Setup (first-run only; routes return 410 once admin exists)
| Method | Path                            | Role | Description                                              |
|--------|---------------------------------|------|----------------------------------------------------------|
| GET    | `/api/setup/status`             | —    | `{ required: bool }` — drives SPA redirect.              |
| POST   | `/api/setup/verify-token`       | —    | Validates one-time token; issues short-lived setup JWT.  |
| POST   | `/api/setup/complete`           | —    | Setup JWT + `{username, password, totp?}` → creates admin. |

### Auth
| Method | Path                       | Role     | Description                                  |
|--------|----------------------------|----------|----------------------------------------------|
| POST   | `/api/auth/login`          | —        | Username + password (+ TOTP if enabled).     |
| POST   | `/api/auth/logout`         | any      | Revoke current refresh token.                |
| POST   | `/api/auth/refresh`        | —        | Rotate refresh + access tokens.              |
| GET    | `/api/auth/me`             | any      | Current user + role.                         |
| POST   | `/api/auth/2fa/enroll`     | any      | Returns TOTP secret + QR.                    |
| POST   | `/api/auth/2fa/verify`     | any      | Activate enrollment with one valid code.     |
| POST   | `/api/auth/2fa/disable`    | any      | Requires current password.                   |

### System
| Method | Path                          | Role      | Description                              |
|--------|-------------------------------|-----------|------------------------------------------|
| GET    | `/api/system/overview`        | viewer+   | One call: cpu%, mem, disks, uptime, load, host. |
| GET    | `/api/system/cpu`             | viewer+   | Per-core %, freq, model.                 |
| GET    | `/api/system/memory`          | viewer+   | total/used/free/cached/swap.             |
| GET    | `/api/system/disks`           | viewer+   | mounts, fs, used/free, S.M.A.R.T. summary. |
| GET    | `/api/system/temperature`     | viewer+   | hwmon sensors, RPi vcgencmd if present.  |
| GET    | `/api/system/processes`       | viewer+   | top-N by CPU/mem.                        |
| POST   | `/api/system/reboot`          | admin     | Confirm token in body.                   |
| POST   | `/api/system/shutdown`        | admin     | Confirm token in body.                   |
| WS     | `/ws/system/stats`            | viewer+   | 1 Hz live overview stream.               |

### Updates
| Method | Path                       | Role      | Description                                  |
|--------|----------------------------|-----------|----------------------------------------------|
| GET    | `/api/updates/list`        | viewer+   | Cached output of `apt list --upgradable`.    |
| POST   | `/api/updates/check`       | operator+ | Run `apt-get update`; returns job id.        |
| POST   | `/api/updates/apply`       | admin     | Run `apt-get -y upgrade`; returns job id.    |
| GET    | `/api/updates/jobs/:id`    | operator+ | Job status + tail of output.                 |
| WS     | `/ws/updates/jobs/:id`     | operator+ | Stream job output.                           |

### Services (systemd)
| Method | Path                              | Role      | Description                              |
|--------|-----------------------------------|-----------|------------------------------------------|
| GET    | `/api/services`                   | viewer+   | List units (filterable).                 |
| GET    | `/api/services/:unit`             | viewer+   | Details + recent journal lines.          |
| POST   | `/api/services/:unit/start`       | operator+ |                                          |
| POST   | `/api/services/:unit/stop`        | operator+ |                                          |
| POST   | `/api/services/:unit/restart`     | operator+ |                                          |
| POST   | `/api/services/:unit/enable`      | admin     |                                          |
| POST   | `/api/services/:unit/disable`     | admin     |                                          |
| WS     | `/ws/services/:unit/logs`         | viewer+   | journalctl -fu unit, tail mode.          |

### Containers (Docker / Podman compatible)
| Method | Path                                | Role      | Description                            |
|--------|-------------------------------------|-----------|----------------------------------------|
| GET    | `/api/containers`                   | viewer+   | List with state, image, ports, mounts. |
| GET    | `/api/containers/:id`               | viewer+   | Inspect.                               |
| POST   | `/api/containers/:id/start`         | operator+ |                                        |
| POST   | `/api/containers/:id/stop`          | operator+ |                                        |
| POST   | `/api/containers/:id/restart`       | operator+ |                                        |
| DELETE | `/api/containers/:id`               | admin     | force=bool query.                      |
| WS     | `/ws/containers/:id/logs`           | viewer+   | Live logs.                             |
| WS     | `/ws/containers/:id/stats`          | viewer+   | Live cpu/mem/net/io.                   |

### Terminal
| Method | Path             | Role   | Description                              |
|--------|------------------|--------|------------------------------------------|
| WS     | `/ws/terminal`   | admin  | PTY session. Initial frame negotiates rows/cols/shell. Per-session audit row. |

### Networking
| Method | Path                                  | Role     | Description                          |
|--------|---------------------------------------|----------|--------------------------------------|
| GET    | `/api/network/interfaces`             | viewer+  | List with MAC, IPs, state, stats.    |
| PATCH  | `/api/network/interfaces/:name`       | admin    | Switch DHCP/static (writes netplan). |
| GET    | `/api/network/firewall`               | viewer+  | UFW status + rules.                  |
| POST   | `/api/network/firewall/rules`         | admin    | Add rule.                            |
| DELETE | `/api/network/firewall/rules/:idx`    | admin    | Remove rule.                         |
| POST   | `/api/network/firewall/enable`        | admin    |                                      |
| POST   | `/api/network/firewall/disable`       | admin    |                                      |

### Logs
| Method | Path                       | Role      | Description                                |
|--------|----------------------------|-----------|--------------------------------------------|
| GET    | `/api/logs/journal`        | viewer+   | Filters: unit, priority, since, cursor.    |
| WS     | `/ws/logs/journal`         | viewer+   | Live tail with same filters.               |

### Settings, Users, Audit
| Method | Path                 | Role   | v0.1?  | Description                          |
|--------|----------------------|--------|--------|--------------------------------------|
| GET    | `/api/settings`      | admin  | yes    | Server-wide prefs (host name, telemetry, TLS info). |
| PATCH  | `/api/settings`      | admin  | yes    |                                      |
| POST   | `/api/settings/password` | admin | yes  | Change own password (requires current). |
| GET    | `/api/audit`         | admin  | yes    | Paged audit log with filters.        |
| GET    | `/api/users`         | admin  | v0.2   |                                      |
| POST   | `/api/users`         | admin  | v0.2   |                                      |
| PATCH  | `/api/users/:id`     | admin  | v0.2   | Role, password reset, TOTP reset.    |
| DELETE | `/api/users/:id`     | admin  | v0.2   |                                      |

### Health / Meta
| Method | Path                 | Role   | Description                          |
|--------|----------------------|--------|--------------------------------------|
| GET    | `/api/healthz`       | —      | 200 if event loop responsive.        |
| GET    | `/api/version`       | any    | Build info, commit, go version.      |

---

## 7. Configuration

All via env (also accepted as `--flag`):

| Var                     | Default                           | Notes                                  |
|-------------------------|-----------------------------------|----------------------------------------|
| `CR_ADDR`               | `:8443`                           | Bind address.                          |
| `CR_DATA_DIR`           | `/var/lib/controlroom`            | SQLite, TLS, audit.                    |
| `CR_TLS_MODE`           | `selfsigned`                      | `selfsigned` \| `acme` \| `proxy`.     |
| `CR_ACME_HOST`          | —                                 | Required when `acme`.                  |
| `CR_ACME_EMAIL`         | —                                 |                                        |
| `CR_TRUST_PROXY`        | `false`                           | Honor `X-Forwarded-*`.                 |
| `CR_LOG_LEVEL`          | `info`                            |                                        |
| `CR_DOCKER_SOCK`        | `/var/run/docker.sock`            | Empty disables container module.       |
| `CR_SESSION_HOURS`      | `168` (7d refresh)                |                                        |
| `CR_HOST_NAME`          | (kernel hostname)                 | Display name in UI title bar.          |
| `CR_VERSION_CHECK`      | `false`                           | Opt-in: daily GitHub release ping.     |

---

## 8. Roadmap (high-level — detail in MILESTONES.md next)

- **M1 — Foundation:** repo skeleton, Go server boots, embed.FS, healthz, CI, Dockerfile.
- **M2 — Auth & setup wizard:** SQLite store, login, JWT cookies, CSRF, browser setup wizard, single-admin enforcement (RBAC scaffold for v0.2).
- **M3 — Dashboard skeleton:** SPA shell, login page, sidebar nav, /system/overview live tile.
- **M4 — Services:** systemd module + UI list, start/stop/restart, live logs over WS.
- **M5 — Containers:** Docker module + UI parity with services.
- **M6 — Terminal:** PTY over WS + xterm.js, audit each session.
- **M7 — Updates:** apt jobs + UI progress.
- **M8 — Networking & logs:** interfaces, firewall, journal browser.
- **M9 — Polish:** mobile pass, accessibility audit, install.sh, docs, v0.1 release.
- **Phase 2 (later):** backups, compose editor, sensors graphing, custom scripts, Discord/Telegram notifications.

---

## 9. Decisions Log

| # | Decision                                                                                  | Rationale                                              |
|---|-------------------------------------------------------------------------------------------|--------------------------------------------------------|
| 1 | **Debian/Ubuntu only at v0.1.** `pkg` interface in code so Fedora/Arch can be added later.| Avoid the apt/dnf/pacman matrix while we ship v0.1.    |
| 2 | **Browser setup wizard, gated by one-time token.**                                        | Best UX without giving the first network scanner admin.|
| 3 | **Single-user (admin only) at v0.1; RBAC in v0.2.**                                       | Cuts ~6 endpoints + role enforcement work from MVP.    |
| 4 | **Telemetry off by default; opt-in version check only.**                                  | Air-gap-friendly; no analytics, no PII.                |
