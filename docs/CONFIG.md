# Configuration reference

Everything ControlRoom can be configured with, plus the per-tab
"what does this need to actually work" matrix.

## Environment variables

All settings are environment variables. There is no on-disk config file —
in bare-metal install they live in `/etc/controlroom/controlroom.env`
(read by the systemd unit), in Docker they go in the `environment:` block
of `docker-compose.yml`, and in the K8s Pod they go in the Deployment's
`spec.template.spec.containers[0].env`.

### Core

| Var | Default | Purpose |
|---|---|---|
| `CR_ADDR` | `:8443` | TCP bind address for the HTTPS listener (or plain HTTP when `CR_DEV=true`). Use `127.0.0.1:8080` when fronting with a TLS-terminating reverse proxy. |
| `CR_DATA_DIR` | `/var/lib/controlroom` | Where TLS material, the JWT signing key, the SQLite DB, and the audit log live. Must be absolute. The directory is created with mode 0750 if missing. |
| `CR_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error`. Controls zerolog filter. |
| `CR_HOST_NAME` | (kernel hostname) | Display name in the SPA title bar. Cosmetic only. |
| `CR_SESSION_HOURS` | `168` (7 days) | Refresh-token lifetime. Access tokens are always 15 min. Reducing this forces more frequent re-logins. |
| `CR_VERSION_CHECK` | `false` | When `true`, ControlRoom does a daily release-check against GitHub releases and surfaces an "update available" badge. Off by default to avoid outbound calls. |
| `CR_DEV` | `false` | Plain HTTP + non-Secure cookies. **Development only** — never set in production. |

### TLS

| Var | Default | Purpose |
|---|---|---|
| `CR_TLS_MODE` | `selfsigned` | `selfsigned` / `acme` / `proxy`. See [`INSTALL.md` → TLS modes](./INSTALL.md#tls-modes). |
| `CR_ACME_HOST` | — | Required when `CR_TLS_MODE=acme`. The hostname Let's Encrypt issues against. DNS must already point at the host. |
| `CR_ACME_EMAIL` | — | Required when `CR_TLS_MODE=acme`. Account email for the ACME directory. |
| `CR_TRUST_PROXY` | `false` | When `true`, honor `X-Forwarded-For` and `X-Forwarded-Proto` for client IP and scheme detection. Set this only when ControlRoom is *actually* behind a trusted reverse proxy — setting it on an exposed server lets clients lie about their IP. |

### Integrations

| Var | Default | Purpose |
|---|---|---|
| `CR_DOCKER_SOCK` | `/var/run/docker.sock` | Unix socket path for the Docker daemon. Set to empty (`""`) to disable the Docker integration entirely (Containers tab gets hidden via the capability flag). |
| `KUBECONFIG` | (none) | Standard kubeconfig path. ControlRoom's K8s client checks: (1) in-cluster SA, (2) `$KUBECONFIG`, (3) `~/.kube/config`, (4) `/etc/rancher/k3s/k3s.yaml`. First match wins. |

### Container-only

These only matter for the Docker deployment. The bare-metal systemd unit
spawns shells directly as the controlroom user; the in-cluster Pod has no
host shell to spawn at all.

| Var | Default | Purpose |
|---|---|---|
| `CR_HOST_SHELL` | `false` | When `true`, the Terminal tab spawns its shell via `nsenter -t 1 -m -u -i -n -p` to enter the host's namespaces. Requires the container to run with `pid: host` and `cap_add: SYS_ADMIN`. The fat-privileged compose sets this `true`. |
| `CR_TERMINAL_LOGIN` | `false` | When `true` (and `CR_HOST_SHELL=true`), the Terminal prompts for username, then `setpriv --reuid=nobody -- su -l <user>`. PAM handles the password prompt; on success drops to that user's shell. **Strongly recommended** whenever `CR_HOST_SHELL=true` — without it you land directly as root inside the host namespaces. |

### Compose helpers

These aren't read by the Go binary — they're consumed by `docker-compose.yml`
expansion only.

| Var | Default | Purpose |
|---|---|---|
| `DOCKER_GID` | `999` | The host's `docker` group GID. Used by `group_add` so the controlroom container can read `/var/run/docker.sock` even when running as a non-root user. Get it with `getent group docker \| cut -d: -f3`. (The default fat-privileged compose runs as root and doesn't strictly need `group_add`, but the var is preserved for users who run a less-privileged variant.) |

---

## Per-tab capability matrix

What each tab needs to function, by deployment shape.

| Tab | Backend integration | Bare-metal | Docker (fat-privileged) | In-cluster Pod |
|---|---|:-:|:-:|:-:|
| **Dashboard** | `/proc`, `/sys` reads | ✅ | ✅ | ✅ host metrics not the cluster |
| **Updates** | `apt`, `apt-get`, `sudo`, `systemctl reboot` | ✅ via sudoers | ✅ binaries in image, host `/etc/apt` + `/var/cache/apt` mounted | ❌ (no apt in cluster) |
| **Services** | dbus + `systemctl` | ✅ via dbus session | ✅ `/var/run/dbus/system_bus_socket` mounted | ❌ |
| **Containers** | `/var/run/docker.sock` | ✅ if controlroom user is in `docker` group | ✅ socket mounted ro | ❌ |
| **Kubernetes** | client-go with kubeconfig or in-cluster SA | ✅ if `kubectl` works as the controlroom user | ✅ `/etc/rancher/k3s/k3s.yaml` mounted | ✅ in-cluster ServiceAccount |
| ↳ Pod **exec** | client-go remotecommand (SPDY) | ✅ | ✅ | ✅ requires `pods/exec: create` in ClusterRole |
| ↳ Lifecycle actions | dynamic-client patch / scale / delete / node-patch | ✅ | ✅ | ✅ requires `patch`/`update`/`delete` per resource |
| ↳ ConfigMap edit | dynamic-client update | ✅ | ✅ | ✅ requires `configmaps: update` |
| ↳ Secret view (read-only) | typed-client get/list, base64 decoded server-side | ✅ | ✅ | ✅ requires `secrets: get,list` |
| ↳ Manifest YAML edit | dynamic-client update with DryRunAll option | ✅ | ✅ | ✅ requires `update` on the editable kind |
| **Terminal** | PTY + (optional) `nsenter`+`su` | ✅ shell as controlroom user | ✅ host shell via nsenter+PAM login | ❌ |
| **Network** | `ip -j addr show`, `sudo ufw …` | ✅ via sudoers | ✅ via `network_mode: host` + NET_ADMIN | ❌ shows Pod's network ns only |
| **Logs** | `journalctl` + (fallback) docker logs | ✅ via `adm` group | ✅ `/run/log/journal` + `/etc/machine-id` mounted | ❌ host journal not reachable |
| **Settings** | SQLite + JWT key | ✅ | ✅ | ✅ |

ControlRoom auto-detects which integrations are working at boot and
exposes the result at `GET /api/system/capabilities`:

```json
{
  "systemd": true,
  "docker": true,
  "journal": true,
  "kubernetes": true
}
```

The SPA reads this on every page load and **hides the nav entries** for
features that aren't reachable. Failed integrations log a warning at
startup but never crash the binary — every integration is best-effort.

---

## Per-shape file paths

Where things live on disk in each shape.

| Path | Bare-metal | Docker | In-cluster Pod |
|---|---|---|---|
| Binary | `/opt/controlroom/controlroom` | `/app/controlroom` (in image) | `/app/controlroom` (in image) |
| Data dir | `/var/lib/controlroom/` | Docker volume `controlroom-data` (mountpoint `/var/lib/docker/volumes/controlroom-data/_data` on host) | PVC `controlroom-data` (1Gi, RWO, `local-path`) |
| SQLite DB | `$DATA_DIR/controlroom.db` | same | same |
| JWT signing key | `$DATA_DIR/jwt.key` (mode 0600) | same | same |
| TLS cert+key | `$DATA_DIR/tls/{server.crt,server.key}` | same | same |
| ACME cache | `$DATA_DIR/acme/` (when `CR_TLS_MODE=acme`) | same | same |
| Systemd unit | `/etc/systemd/system/controlroom.service` | n/a | n/a |
| Sudoers fragment | `/etc/sudoers.d/controlroom` | n/a (root in container) | n/a (Pod, no sudoers) |
| Env file | `/etc/controlroom/controlroom.env` | inline in compose `environment:` | inline in Deployment env |
| Logs | `journalctl -u controlroom` | `docker logs controlroom` | `kubectl logs -n controlroom deployment/controlroom` |

---

## Capability detection — debugging

If a tab is hidden in the SPA and you expected it to work:

1. Look at the boot log. ControlRoom logs one warning per failed
   integration at startup:
   - `systemd unavailable; /api/services disabled`
   - `journalctl not in PATH; /api/logs/journal will return 503`
   - `docker unavailable; /api/containers disabled`
   - `kubernetes unavailable; /api/k8s disabled`

2. Hit the capabilities endpoint as a logged-in user (you can grab the
   cookie from devtools and curl):
   ```
   curl -sk -b "cr_access=<token>" https://<host>:8443/api/system/capabilities
   ```
   The booleans tell you what the SPA is using to gate the nav.

3. For Docker-shape deployments specifically, double-check the bind mount
   actually exists inside the container:
   ```
   docker exec controlroom ls -la /var/run/docker.sock
   docker exec controlroom ls -la /var/run/dbus/system_bus_socket
   docker exec controlroom ls -la /etc/k3s/kubeconfig
   docker exec controlroom which journalctl systemctl ip ufw apt
   ```

4. For the Kubernetes tab specifically, run a probe from inside the
   container:
   ```
   docker exec controlroom sh -c '
     KUBECONFIG=/etc/k3s/kubeconfig kubectl version --client
     KUBECONFIG=/etc/k3s/kubeconfig kubectl get nodes 2>&1 | head
   '
   ```

---

## Compose env-file pattern

Rather than putting all `CR_*` vars in `docker-compose.yml`, you can write
an `.env` file next to it:

```ini
# .env
CR_TLS_MODE=acme
CR_ACME_HOST=ctrl.example.com
CR_ACME_EMAIL=admin@example.com
DOCKER_GID=999
```

Compose auto-loads it. The `.env` file is `.gitignore`d by default in
this repo.

For bare-metal, the equivalent is `/etc/controlroom/controlroom.env` —
the systemd unit reads it via `EnvironmentFile=`. Same syntax (`KEY=value`,
no quotes needed).
