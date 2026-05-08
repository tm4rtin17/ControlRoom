# ControlRoom — Development Plan

> Companion to `SPEC.md`. Nine milestones from "empty repo" to "v0.1 released."
> Each milestone is small enough to ship in 1–3 focused sessions and ends in a runnable, demoable state.

**Conventions**
- ✅ = acceptance criterion (must be true to call the milestone done).
- 🧪 = test added in this milestone.
- 📝 = doc added/updated.
- Every milestone ends with a tagged commit (`m1-foundation`, `m2-auth`, …).

---

## Cross-cutting standards (apply to every milestone)

- **Errors:** all API responses follow `{ "error": { "code": "...", "message": "...", "detail": {...}? } }` on non-2xx. Codes are stable strings (e.g. `auth.invalid_credentials`).
- **Logging:** zerolog JSON to stderr; one log line per request via middleware (`method path status duration_ms user_id`). Privileged actions also log to `audit_log` table.
- **Validation:** zod schemas in `web/src/lib/schemas/` mirror Go struct tags; backend validates on entry, frontend validates before submit.
- **Tests:** Go unit tests for every package with logic; integration test in `internal/api/router_test.go` boots the server in-process. Vitest for frontend lib code; Playwright smoke test added in M9.
- **CI:** golangci-lint, `go test ./...`, `eslint`, `vitest run`, build the Docker image. Required for merge.

---

## M1 — Foundation

**Goal:** repo skeleton boots a single binary that serves a placeholder SPA over HTTPS.

### Tasks
1. Init `go.mod` (`github.com/<owner>/controlroom`, Go 1.23).
2. Scaffold folder tree per `SPEC.md` §4.
3. `cmd/controlroom/main.go`: env+flag parsing, signal handling, graceful shutdown.
4. `internal/config/`: typed config struct, defaults, validation.
5. `internal/api/router.go`: Fiber v3 app with logger + recover + request-id middleware.
6. `internal/tls/`: self-signed cert generator (writes to `$DATA_DIR/tls/`); ACME and proxy modes stubbed with TODO panics so they're explicit.
7. `embed.FS` mount of `web/dist` at `/`. SPA fallback: any non-`/api`/`/ws` route returns `index.html`.
8. Endpoints: `GET /api/healthz`, `GET /api/version` (build info via `-ldflags`).
9. Vite + React + TS + Tailwind + shadcn/ui scaffold in `web/`. Add `Button`, `Card`, `Input` shadcn components. Dark theme as default.
10. Splash page: logo + "ControlRoom — connecting…" with one fetch to `/api/version`.
11. `Makefile`: `make dev` (concurrent vite+go-run with hot reload via `air`), `make build`, `make image`, `make lint`, `make test`.
12. `deploy/Dockerfile`: 3 stages — node-build → go-build (CGO_ENABLED=0) → `gcr.io/distroless/static-debian12:nonroot`.
13. `.github/workflows/ci.yml`: lint, test, build image (no push).
14. `README.md` (minimal — quickstart + status badge).

### Acceptance
- ✅ `make image && docker run -p 8443:8443 controlroom:dev` → splash page over self-signed HTTPS at `https://localhost:8443`.
- ✅ `curl -sk https://localhost:8443/api/healthz` → `200 {"status":"ok"}`.
- ✅ Image size ≤ 30 MB; idle RSS ≤ 50 MB (`docker stats`).
- ✅ CI green on a PR.
- 🧪 `internal/config/config_test.go`: defaults + invalid combos.
- 📝 `README.md` with quickstart.

**Tag:** `m1-foundation`.

---

## M2 — Auth & setup wizard

**Goal:** secure, single-admin auth working end-to-end. Fresh install → browser wizard → login → authenticated dashboard placeholder.

### Tasks
1. `internal/store/`: open SQLite via `modernc.org/sqlite`, hand-rolled migrations table + numbered SQL files in `internal/store/migrations/`.
2. Migrations 001:
   - `users(id, username UNIQUE, password_hash, totp_secret NULL, totp_enabled BOOL, role DEFAULT 'admin', created_at, updated_at)`
   - `sessions(id, user_id FK, refresh_hash, family_id, parent_id NULL FK, ip, user_agent, created_at, expires_at, revoked_at NULL)`
   - `audit_log(id, ts, user_id NULL, ip, action, target NULL, outcome, detail JSON)`
   - `setup_token(token_hash, created_at, used_at NULL)` — single row, generated on empty users table.
3. `internal/auth/password.go`: bcrypt(cost=12) wrap, constant-time compare on verify.
4. `internal/auth/totp.go`: enroll (generate secret + QR PNG bytes), verify (RFC 6238, ±1 step window).
5. `internal/auth/jwt.go`: HS256 access (15m) + refresh (7d), key from `$DATA_DIR/jwt.key` (rotated on bootstrap if missing).
6. `internal/auth/sessions.go`: rotating refresh with family tracking — reuse of a revoked refresh in a family revokes the whole family + writes critical audit event.
7. Middleware:
   - `middleware/auth.go` — required-auth wrapper; reads access cookie, attaches `*User` to ctx.
   - `middleware/csrf.go` — double-submit token check on POST/PATCH/DELETE.
   - `middleware/ratelimit.go` — token bucket per IP; `/api/auth/login` 5/min, exponential backoff per username (in-memory map; persists across restarts not required for v0.1).
   - `middleware/audit.go` — wraps state-changing handlers, writes audit row.
8. Handlers:
   - `setup`: `GET /status`, `POST /verify-token` (returns short-lived setup JWT in cookie), `POST /complete` (creates admin, optionally TOTP, marks token used).
   - `auth`: `login`, `logout`, `refresh`, `me`, `2fa/enroll`, `2fa/verify`, `2fa/disable`.
9. Bootstrap: on server start, if `users` empty, generate one-time setup token (32 random bytes hex), insert hash, log it once at INFO with `==== ControlRoom setup token: <token> ====`.
10. Frontend:
    - `lib/api.ts` typed fetch (auto-refresh on 401, CSRF header injection).
    - `lib/auth.ts` TanStack Query hooks: `useMe`, `useLogin`, `useLogout`.
    - Routes: `/setup` (3-step wizard with shadcn `Stepper` + `Form` + `OTPInput`), `/login`, `/` (placeholder authenticated landing).
    - Top-level guard: route to `/setup` if `GET /api/setup/status` returns `required: true`; route to `/login` on 401; else render app.
    - Strong password meter (zxcvbn-ts, lazy-loaded).

### Acceptance
- ✅ Fresh container boots → token printed once → browser at `https://host:8443` → auto-redirect to `/setup` → token check → username + password → optional TOTP enrollment → done → redirected to `/login` → log in → see authenticated placeholder.
- ✅ Reusing a refresh token revokes its family; client is forced back to login.
- ✅ Wrong password 5× from same IP → 429 for 60s.
- ✅ `/api/auth/me` reflects active TOTP state.
- ✅ `/setup` returns 410 once admin exists.
- 🧪 `auth/password_test.go`, `auth/totp_test.go`, `auth/sessions_test.go` (rotation + reuse), `api/auth_test.go` (full login flow), `api/setup_test.go`.
- 📝 `docs/SECURITY.md` first draft (threat model + auth model).

**Tag:** `m2-auth`.

---

## M3 — Dashboard skeleton + system overview

**Goal:** authenticated user sees a live, mobile-friendly dashboard with real metrics.

### Tasks
1. `internal/collectors/`:
   - `cpu.go` — `/proc/stat` sampling at 1Hz with delta (per-core %).
   - `mem.go` — `/proc/meminfo` (total/used/free/cached/buffers/swap).
   - `disks.go` — parse `/proc/mounts`, statfs each, exclude pseudo-fs; opportunistic SMART summary via `smartctl -j` if installed.
   - `net.go` — `/proc/net/dev` (RX/TX counters → rates).
   - `temps.go` — `sysfs hwmon` walk; if `vcgencmd` exists (RPi), include `measure_temp`.
   - `host.go` — uname, distro from `/etc/os-release`, kernel, uptime, load.
   - All collectors implement `Snapshot() (T, error)`; a top-level `Aggregator` runs them concurrently.
2. Endpoints: `GET /api/system/overview`, `WS /ws/system/stats` (1Hz, JSON-encoded snapshots, ping/pong every 30s).
3. SPA shell:
   - `components/AppLayout.tsx`: sidebar (Dashboard / Services / Containers / Terminal / Logs / Network / Settings), top bar (host name, status pill), mobile drawer toggle.
   - Theme toggle (light/dark/system) persisted in localStorage; CSS vars driven.
   - Polished empty/loading/error states.
4. Dashboard tiles (responsive 2-col on mobile, 4-col desktop):
   - CPU (overall % + per-core mini bars, model + freq).
   - Memory (used/total + swap, mini chart).
   - Disks (per-mount bar, SMART pill if present).
   - Temperature (largest reading + sensor list popover).
   - Uptime + Load.
   - Network (RX/TX rate per interface).
5. Live updates via `useWebSocket` hook pushing into TanStack Query cache.

### Acceptance
- ✅ Dashboard updates every 1s on desktop and mobile (Chrome DevTools throttled to "Slow 4G" still works).
- ✅ Lighthouse mobile score ≥ 90 on the dashboard.
- ✅ WS reconnects within 5s after network blip; falls back to REST polling at 5s if WS fails.
- ✅ No layout shift on metric updates (reserve dimensions).
- 🧪 `collectors/*_test.go` with fixture `/proc` files in `testdata/`.
- 🧪 `api/system_test.go` snapshot test of the overview shape.

**Tag:** `m3-dashboard`.

---

## M4 — Services (systemd)

**Goal:** list and control systemd units; live logs.

### Tasks
1. `internal/systemd/` — interface `SystemD` with implementations `dbus` (real) and `fake` (for tests).
   - `List(filter)`, `Get(unit)`, `Start/Stop/Restart/Enable/Disable(unit)`.
   - Unit name validation regex (`^[a-zA-Z0-9@_.\-:\\]+\.(service|socket|target|timer|path|mount)$`).
2. Logs: `WS /ws/services/:unit/logs` shells out to `journalctl -fu <unit> --output=short-iso -n 200` and streams; cancellation kills the child process.
3. Endpoints per SPEC §6.
4. SPA `/services` page:
   - Sortable, searchable virtualized table (TanStack Table + Virtual). Columns: Name, Description, Load, Active, Sub, Memory, Tasks.
   - Inline action menu (start/stop/restart/enable/disable with confirmation modals where destructive).
   - Slide-over (`Sheet`) on row click: details + last-N journal + live tail toggle.
   - Status pills: active=green, inactive=gray, failed=red, activating/deactivating=yellow.
5. Audit each action.

### Acceptance
- ✅ List of ≥ all `*.service` on a fresh Ubuntu, sorts/searches without lag at 500 units.
- ✅ Start/stop/restart works; UI updates within 1s of action.
- ✅ Live log tail keeps up with a chatty unit (`journalctl -f` test).
- ✅ Invalid unit name from URL → 400, never executed.
- 🧪 `systemd/dbus_test.go` (skipped in CI without dbus); `systemd/fake_test.go`; `api/services_test.go` using fake.

**Tag:** `m4-services`.

---

## M5 — Containers (Docker)

**Goal:** parity with services, but for Docker/Podman containers.

### Tasks
1. `internal/docker/` — wrapper around `docker/docker/client`. Auto-detect socket; if missing, module disables itself and `/api/containers` returns 503 with `code: "containers.unavailable"` (SPA hides nav entry).
2. Endpoints per SPEC §6 (list, inspect, start/stop/restart, delete, WS logs, WS stats).
3. Stats: subscribe to docker stats stream; downsample to 1Hz before WS forward.
4. SPA `/containers`:
   - Toggle: grid (cards) ↔ table.
   - Group by `com.docker.compose.project` label with collapsible headers.
   - Card shows: image, status, port mappings, CPU%, MEM, uptime.
   - Slide-over: full inspect, env vars (masked toggle), mounts, live logs, live stats sparklines.
   - Action confirmations; "force" checkbox on delete.
5. Audit each action.

### Acceptance
- ✅ Works against Docker Desktop, Docker Engine, and rootless Podman with the docker compat socket.
- ✅ Stats stream stays under 5% CPU when watching 30 containers.
- ✅ Removing the docker socket and reloading hides the page cleanly (no 500s).
- 🧪 `docker/client_test.go` against ephemeral docker via `dockertest`; `api/containers_test.go` with a fake.

**Tag:** `m5-containers`.

---

## M6 — Terminal

**Goal:** secure, fast, full PTY in the browser.

### Tasks
1. `internal/pty/` session manager:
   - `New(rows, cols, shell, env)` → spawns shell with `creack/pty`, returns session.
   - Bidirectional `io.Copy` with backpressure-aware buffering (32 KB).
   - `Resize`, `Close` (sends SIGHUP, waits, then SIGKILL after 5s).
   - Configurable allowed shells (`/bin/bash`, `/bin/sh`, `/usr/bin/zsh`); unknown rejected.
2. `WS /ws/terminal`:
   - First client frame: JSON `{rows, cols, shell?}`.
   - Subsequent: binary frames raw to PTY; control frames JSON `{type:"resize", rows, cols}`.
   - Server → client: binary frames raw from PTY.
   - Audit start (`session_id, shell`) and end (`exit_code, duration_ms, bytes_in, bytes_out`); **no keystroke recording**.
3. SPA `/terminal`:
   - xterm.js + FitAddon + WebLinksAddon + ClipboardAddon.
   - Theme matches app palette.
   - Reconnect button on disconnect (creates fresh session).
   - Mobile: virtual keyboard tap to focus; ctrl+alt soft-buttons row.
4. Idle timeout: server closes session after 30 min idle (configurable).

### Acceptance
- ✅ Open terminal → run `top` → resize browser → output reflows immediately.
- ✅ Closing browser tab terminates the shell within 5s (verified with `pgrep`).
- ✅ Pasting 10 KB block doesn't desync.
- ✅ Audit log shows session start + end with byte counts.
- 🧪 `pty/manager_test.go`, `api/terminal_test.go` (golang test driving WS with `nhooyr/websocket`).

**Tag:** `m6-terminal`.

---

## M7 — Updates (apt)

**Goal:** check & apply system updates with live progress.

### Tasks
1. `internal/jobs/` — generic background job runner (concurrent jobs by id; ring-buffer per-job stdout/stderr; final outcome persisted).
2. `internal/updates/apt.go`:
   - `Upgradable()` parses `apt list --upgradable` (sets locale `LC_ALL=C`).
   - `Update()` runs `sudo apt-get update`.
   - `Upgrade()` runs `sudo apt-get -y -o Dpkg::Options::="--force-confold" upgrade`.
3. Endpoints per SPEC §6: list, check, apply, jobs/:id, WS jobs/:id.
4. SPA `/updates`:
   - Package table (name, current → new, source, size, security pill).
   - "Check for updates" button → starts job, opens progress drawer.
   - "Install updates" → modal listing packages + estimated size → confirm → progress drawer with live tail.
   - Reboot-required banner if `/var/run/reboot-required` exists; one-click reboot (with explicit confirm input "REBOOT").

### Acceptance
- ✅ End-to-end on a Debian/Ubuntu VM with held-back packages and security updates.
- ✅ Job survives client disconnect; reconnecting shows full output to-date.
- ✅ Two simultaneous `apply` calls → second returns 409.
- 🧪 `updates/apt_test.go` parses fixture outputs of `apt list --upgradable`.

**Tag:** `m7-updates`.

---

## M8 — Networking & logs

**Goal:** view/edit interfaces and firewall; browse and tail journald.

### Tasks
**Networking**
1. `internal/network/iface.go`: enumerate via `vishvananda/netlink`; live RX/TX deltas.
2. `internal/network/netplan.go`: read `/etc/netplan/*.yaml`, present current config; `PATCH` writes a new file `90-controlroom.yaml`, runs `netplan try` (10s revert) then `netplan apply` if confirmed in-UI.
3. `internal/network/ufw.go`: parse `ufw status numbered`; add/delete/enable/disable.
4. SPA `/network`:
   - Interfaces card (per iface: name, state, MAC, IPs, RX/TX rate, MTU).
   - Edit slide-over: DHCP / static (IP/CIDR, gateway, DNS) with `netplan try` warning.
   - Firewall card: status toggle, rules table with add-rule form.

**Logs**
5. `internal/logs/journal.go`: shell out to `journalctl --output=json` for both static (with `-S`/`-U`/`-p`/`-u`/`-n`) and live tail (`-f`).
6. SPA `/logs`:
   - Filter bar (unit autocomplete, priority dropdown, since picker, free-text search).
   - Virtualized list with priority color-coding.
   - "Live tail" toggle → switches to WS.
   - Each row expandable for full JSON.

### Acceptance
- ✅ Switch interface from DHCP→static and back without losing the SSH session that's serving ControlRoom (via `netplan try` revert).
- ✅ Add and remove a UFW rule; rule list refreshes immediately.
- ✅ Logs page handles 10k lines in the virtualized list smoothly.
- ✅ Filtering by unit + priority + search composes correctly.
- 🧪 `network/netplan_test.go` (parse + emit YAML), `network/ufw_test.go` (parse `ufw status numbered` fixtures), `logs/journal_test.go` (parse JSON output).

**Tag:** `m8-net-logs`.

---

## M9 — Polish, install, release

**Goal:** v0.1 ships. Anyone can install it on a fresh Ubuntu Server in under 5 minutes.

### Tasks
1. **Mobile QA** — every page used one-handed on a phone; test on iOS Safari + Android Chrome.
2. **Accessibility** — axe-core via Playwright; keyboard nav full coverage; aria-live for status changes; contrast ≥ AA.
3. **Public-bind warning** — banner if server bound to non-loopback non-RFC1918 address.
4. **Settings page** finalized:
   - Account: change password, manage TOTP.
   - Server: host display name, telemetry opt-in toggle, TLS info, log level.
   - About: version, commit, runtime info.
5. **`deploy/install.sh`** (bare-metal):
   - Detects Debian/Ubuntu; aborts elsewhere with a clear message.
   - Creates `controlroom` user, `$DATA_DIR`.
   - Downloads release binary (or builds locally if `--from-source`).
   - Writes `/etc/systemd/system/controlroom.service` with capability + `ProtectSystem=strict`.
   - Writes `/etc/sudoers.d/controlroom` (validated with `visudo -c`).
   - Generates self-signed cert; prints first-run token.
   - `--uninstall` reverses cleanly.
6. **`deploy/docker-compose.yml`** with sensible defaults + comments.
7. **Docs:**
   - `README.md` — hero, screenshots, quickstart (Docker + bare-metal), feature list, link to SPEC.
   - `docs/INSTALL.md` — both paths in detail.
   - `docs/SECURITY.md` — finalize threat model + reporting.
   - `docs/API.md` — generated from handler annotations.
8. **Release workflow** — `goreleaser` config builds linux/amd64 + linux/arm64 binaries + multi-arch Docker image; pushes on git tag.
9. **Smoke test** — Playwright run on a real Ubuntu Server VM hitting the major flows.

### Acceptance
- ✅ Fresh Ubuntu 24.04 Server VM → `curl -fsSL <install-url> | sudo bash` → wizard → dashboard → start/stop a service → live terminal → `apt upgrade` → done. Total time: target ≤ 5 minutes.
- ✅ Idle RSS ≤ 50 MB after 1 hour with 5 dashboard clients connected.
- ✅ Image size ≤ 25 MB.
- ✅ axe-core: 0 critical, 0 serious.
- ✅ All public-facing docs cross-link correctly.
- 🧪 Playwright smoke spec covers: setup wizard, login, dashboard live-update, start/stop a service, open terminal and run `echo hi`, list containers (or empty-state if none).

**Tag:** `v0.1.0`.

---

## Phase 2 backlog (post-v0.1)

Not scheduled; sized roughly so we can prioritize later.

- **RBAC** — promote `users.role` to enforced; add user CRUD UI + endpoints. (~M2-sized)
- **Backups** — restic integration (snapshots, schedules, restore). (~M-large)
- **Compose editor** — visual + raw editor for `docker-compose.yml`, validate + apply. (~M-large)
- **Sensors graphing** — historical metrics in a small TSDB (Prometheus remote_write or local rrd-style). (~M-large)
- **Custom scripts** — admin-defined buttons that run a vetted command with a confirmation. (~M-medium)
- **Notifications** — Discord/Telegram/Webhook on selected audit events (failed logins, service crash, update available). (~M-medium)
- **Multi-host federation** — register peer ControlRoom instances; aggregated dashboard. (~M-extra-large; arguably its own product.)
- **Distro expansion** — `dnf` and `pacman` backends behind the existing `pkg` interface. (~M-medium each)
