# ControlRoom

Lightweight, modern web UI for managing headless Linux homelab servers.

> **Status:** v0.1 in development. The full feature set from the
> [`SPEC.md`](./SPEC.md) is implemented through milestone M9; see
> [`MILESTONES.md`](./MILESTONES.md) for what's done and what's deferred.

## What is it?

A single Go binary that serves a Vite/React SPA over HTTPS and replaces
routine SSH for:

- **Dashboard** — live CPU, memory, disks, temperatures, network rates,
  uptime, load (1 Hz over WebSocket).
- **Updates** — `apt list --upgradable`, one-click check + apply with live
  job streaming and a reboot-required banner.
- **Services** — list / start / stop / restart / enable / disable systemd
  units; live `journalctl -fu` log tail.
- **Containers** — Docker / Podman list with Compose-project grouping; per
  container live logs + CPU/MEM stats; start / stop / restart / delete.
- **Terminal** — full PTY in the browser via xterm.js.
- **Network** — read-only interfaces + UFW rules editor.
- **Logs** — journald browser with filters and live tail.
- **Settings** — change password, manage 2FA, view server config.

## Footprint targets

| Metric          | Goal     |
|-----------------|----------|
| Idle RSS        | ≤ 50 MB  |
| Image size      | ≤ 25 MB  |
| First paint     | < 1 s    |

## Install

### Bare-metal (Debian / Ubuntu)

```bash
sudo deploy/install.sh
sudo journalctl -u controlroom -n 200 | grep setup_token
```

Visit `https://<host>:8443`, paste the setup token, create your admin
account, optionally enable 2FA.

Full guide: [`docs/INSTALL.md`](./docs/INSTALL.md).

### Docker

```bash
docker compose -f deploy/docker-compose.yml up -d
docker compose logs --tail 200 | grep setup_token
```

Note: the container can't reach the host's systemd dbus or journald, so the
**Services** and **Logs** pages will return 503 inside Docker. Bare-metal
install is required for those.

## Develop

Two terminals:

```bash
make dev-api    # Go backend with hot reload (requires `air`)
make dev-web    # Vite dev server
```

Open <http://localhost:5173>. Vite proxies `/api` and `/ws` to the Go backend
on `:8443` (CR_DEV mode → plain HTTP, non-Secure cookies).

Prerequisites: Go 1.23+, Node 20+, [`air`](https://github.com/air-verse/air)
for backend hot reload.

## Project layout

```
cmd/controlroom/                  # entrypoint
internal/                         # backend packages
  api/                            # HTTP routes + middleware
    {auth,containers,logs,network,services,settings,setup,system,terminal,updates}/
  auth/                           # password, TOTP, JWT, sessions, ratelimit
  cert/                           # self-signed + ACME (autocert)
  collectors/                     # /proc and /sys readers
  config/                         # env-based config
  docker/                         # Docker client wrapper
  jobs/                           # generic job runner with ring buffer
  logs/                           # journalctl wrapper
  network/                        # ip + ufw wrappers
  pty/                            # PTY session manager
  store/                          # SQLite migrations + accessors
  systemd/                        # dbus client wrapper
  web/                            # embed.FS for the SPA
web/                              # Vite + React + TS + Tailwind + shadcn/ui
deploy/                           # Dockerfile, install.sh, systemd unit, sudoers, compose
docs/                             # SECURITY.md, INSTALL.md
```

## Security

Single trusted operator, LAN-first. Read [`docs/SECURITY.md`](./docs/SECURITY.md)
for the threat model. Highlights:

- bcrypt(cost 12) + optional TOTP.
- HS256 access JWTs (15 min) + opaque rotating refresh tokens (7 days) with
  reuse detection that revokes the entire token family.
- HttpOnly + Secure + SameSite=Strict cookies; CSRF double-submit token.
- Per-IP rate limit + per-user exponential backoff on login.
- Hardened systemd unit (`NoNewPrivileges`, `ProtectSystem=strict`, system
  call filtering); tight `/etc/sudoers.d/controlroom` fragment validated
  before install.
- Full audit log of every privileged action. **Keystrokes are never recorded.**

Always behind a VPN, SSH tunnel, or reverse proxy with strict firewall rules
when accessing remotely.

## License

MIT — see [`LICENSE`](./LICENSE).
