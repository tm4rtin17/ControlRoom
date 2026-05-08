# Installing ControlRoom

ControlRoom v0.1 supports Debian and Ubuntu (LTS releases). Two install paths
are supported: a one-shot bare-metal installer and a Docker Compose deployment.

## Prerequisites

- Debian 12+ or Ubuntu 22.04+ (older releases haven't been tested).
- A user with `sudo` (only for installation; ControlRoom itself runs as a
  dedicated `controlroom` system user afterwards).
- Optional but recommended: `ufw`, `journalctl` (always present), and Docker
  if you want to manage containers.

## Option A — bare-metal (recommended for full feature set)

The installer creates a `controlroom` system user, installs the binary under
`/opt/controlroom`, persists data in `/var/lib/controlroom`, drops a hardened
systemd unit, and writes a tight sudoers fragment after validating it with
`visudo -c`.

```bash
sudo deploy/install.sh
```

Or, if you cloned the repo and want to build from source first:

```bash
sudo deploy/install.sh --from-source
```

When the service starts it logs a one-time **setup token** — the installer
tails it for you, but you can also run:

```bash
sudo journalctl -u controlroom -n 200 | grep setup_token
```

Visit `https://<host>:8443` (the self-signed cert will be flagged by your
browser the first time), paste the token into the wizard, create your admin
account, optionally enable 2FA, and you're in.

### Uninstall

```bash
sudo deploy/install.sh --uninstall
```

The data directory at `/var/lib/controlroom` is preserved by default — delete
it manually if you want a truly fresh install.

## Option B — Docker Compose

Container deployment is simpler but loses access to the host's systemd dbus
and journald. The `Services` and `Logs` pages will return 503; everything
else works.

```bash
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml logs --tail 200 | grep setup_token
```

To manage other containers, mount the Docker socket read-only (the default
compose file already does this).

## TLS modes

| `CR_TLS_MODE` | Behaviour                                                     |
|---------------|----------------------------------------------------------------|
| `selfsigned`  | (default) Generates a 10-year ECDSA self-signed cert at first boot under `$DATA_DIR/tls/`. |
| `acme`        | Uses Let's Encrypt via HTTP-01. Requires `CR_ACME_HOST` and `CR_ACME_EMAIL`. The host must be reachable on **port 80** from the public internet during issuance and renewal. |
| `proxy`       | Stub for "TLS terminated by an upstream reverse proxy." Bind plain HTTP on the loopback (e.g. `CR_ADDR=127.0.0.1:8080`) and let the proxy speak HTTPS. |

Pair `acme` with `--ports 80:80 8443:443` (or your reverse proxy) and ensure
DNS for `CR_ACME_HOST` already resolves to the host before starting.

## Configuration reference

All settings are environment variables. Persist them in
`/etc/controlroom/controlroom.env` (read by the systemd unit) for bare-metal,
or pass them to `docker compose` via `environment:`.

| Var                  | Default                       | Purpose                                  |
|----------------------|-------------------------------|-------------------------------------------|
| `CR_ADDR`            | `:8443`                        | TLS bind address.                         |
| `CR_DATA_DIR`        | `/var/lib/controlroom`         | TLS material, SQLite, audit log.          |
| `CR_TLS_MODE`        | `selfsigned`                   | `selfsigned` \| `acme` \| `proxy`.        |
| `CR_ACME_HOST`       | —                              | Required when `acme`.                     |
| `CR_ACME_EMAIL`      | —                              | Required when `acme`.                     |
| `CR_TRUST_PROXY`     | `false`                        | Honour `X-Forwarded-*` (proxy mode).      |
| `CR_LOG_LEVEL`       | `info`                         | `debug` \| `info` \| `warn` \| `error`.   |
| `CR_DOCKER_SOCK`     | `/var/run/docker.sock`         | Empty disables container management.      |
| `CR_SESSION_HOURS`   | `168` (7d)                     | Refresh-token lifetime.                   |
| `CR_HOST_NAME`       | (kernel hostname)              | Display name in the SPA title bar.        |
| `CR_VERSION_CHECK`   | `false`                        | Opt-in: daily release check on GitHub.    |
| `CR_DEV`             | `false`                        | Plain HTTP + non-Secure cookies. Dev only.|

## After install

1. Read [`SECURITY.md`](./SECURITY.md) for the threat model and what's
   actually enforced today.
2. Lock down inbound access. ControlRoom is not designed to be exposed
   directly to the public internet — put it behind a VPN, SSH tunnel, or
   reverse proxy with strict firewall rules.
3. Enable two-factor authentication (Settings → Two-factor authentication)
   on every admin account.
