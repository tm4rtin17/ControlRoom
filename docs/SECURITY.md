# Security model — v0.1

This document describes ControlRoom's threat model and the implementation
choices made through v0.1. It tracks what's *actually shipped*; for the
forward-looking design, see [`../SPEC.md`](../SPEC.md) §5.

## Threat model

ControlRoom is designed for a single trusted operator (or a small team) on
a LAN, with optional remote access via a reverse proxy or VPN. **It is not
designed to be exposed raw to the public internet.**

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
| TOTP | RFC 6238, 6 digits / 30 s / SHA-1, ±1-step verify (`internal/auth/totp.go`). Optional. |
| Access token | HS256 JWT, 15 min TTL. Key in `$DATA_DIR/jwt.key` (mode 0600). |
| Refresh token | Opaque `<session_id>.<32-byte-secret>`; only sha256 stored in `sessions.refresh_hash`. 7 day TTL. |
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

## Privilege scoping (bare-metal install)

The systemd unit (`deploy/controlroom.service`) runs as a dedicated
`controlroom` system user with:

- `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`,
  `PrivateTmp`, `PrivateDevices`, kernel-tunables/modules/cgroups protections.
- `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK`.
- `SystemCallFilter=@system-service & ~@privileged @resources`.
- `SupplementaryGroups=adm docker` so it can read the journal and talk to the
  Docker socket when present.

A tight `/etc/sudoers.d/controlroom` fragment NOPASSWDs only the exact
commands ControlRoom needs (apt update/upgrade, systemctl reboot, the UFW
verbs used by `/api/network/firewall/*`). Everything else still requires the
operator to authenticate.

The fragment is validated with `visudo -c` before installation —
`install.sh` aborts on parse failure.

## Audit

Every privileged action writes a row to `audit_log` (best-effort; never fails
the parent request):

- Auth: login success/failure (with reason), logout, refresh, refresh-reuse,
  TOTP enable/disable, password change.
- Setup: token verify, complete (with `totp_enabled`).
- Services / containers: each lifecycle action with target + outcome.
- Updates: check / apply / reboot job starts.
- Firewall: rule add/delete + enable/disable with the rule spec.
- Terminal: `session_start` and `session_end` (duration, bytes_in, bytes_out,
  exit code). Keystrokes are **never** recorded.

Retention is unbounded today — see "Known gaps" below.

## TLS

Three modes:

- `selfsigned` — generated at first boot under `$DATA_DIR/tls/`, ECDSA P-256,
  10-year validity, SAN entries for `localhost`, the kernel hostname, and
  every non-loopback IPv4 on the host.
- `acme` — Let's Encrypt via HTTP-01 (`golang.org/x/crypto/acme/autocert`),
  cache in `$DATA_DIR/acme/`. Port 80 must be reachable.
- `proxy` — bind plain HTTP on loopback and front with a TLS-terminating
  reverse proxy.

In every mode, the cipher suite list excludes anything below TLS 1.2 and the
deprecated CBC/RC4/3DES suites. TLS 1.3 uses the std-lib's fixed list.

## Known gaps (post-v0.1)

These are documented and tracked, not silently missing:

- **Settings persistence**: host display name and the version-check toggle
  are read from env only — no UI write yet. Comes with v0.2 alongside RBAC.
- **API integration tests** for the auth/setup flows. Unit tests cover the
  security-critical pieces (rotation, reuse, password, TOTP, ratelimit
  validation).
- **Audit log retention/rotation**. Grows unbounded today.
- **HSTS / CSP headers**. Planned for the next polish pass.
- **Netplan editing**. Read-only interfaces today.
- **Public-bind detection** in the SPA is best-effort (RFC 1918 heuristic on
  the URL bar).

## Reporting

Pre-1.0: open a GitHub issue with the `security` label, or email the
maintainer if you need to disclose privately.
