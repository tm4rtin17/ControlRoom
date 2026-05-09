# Installing ControlRoom

ControlRoom ships in three deployment shapes. Pick the one that matches your
constraints — they're not mutually exclusive (you can run, e.g., a bare-metal
ControlRoom on the host *and* an in-cluster ControlRoom in K3s for the
cluster-management UI).

## Choosing a shape

| | Bare-metal | Docker container | In-cluster K8s Pod |
|---|---|---|---|
| **Where it runs** | Host, as a systemd unit | Host, as a Docker container | A Pod in your K3s/k8s cluster |
| **Privilege** | Dedicated `controlroom` user + scoped sudoers | Root inside container, host PID/network namespaces, broad capabilities | Nonroot (uid 65532), `drop: [ALL]`, scoped ClusterRole |
| **Image size** | ~20 MB binary + ~1 MB SPA | ~230 MB | ~230 MB |
| **Idle RSS** | ~40 MB | ~50 MB | ~50 MB |
| **Tabs that work** | All (Dashboard, Updates, Services, Containers, Kubernetes\*, Terminal, Network, Logs, Settings) | All | Kubernetes + Dashboard + Settings only — no host integrations |
| **Best for** | Single-host homelab, smallest blast radius, the canonical path. | Single-user homelabs that want to manage the whole host *and* keep ControlRoom in a container. | Cluster operations only; pair with bare-metal/container ControlRoom for host management. |

\* Kubernetes tab on bare-metal works if `kubectl` finds a kubeconfig — see
the [Kubernetes integration](#kubernetes-integration) section.

> **Security note.** The Docker container shape is *fat-privileged*: a
> compromise of the ControlRoom binary inside the container is effectively
> root on the host. That's the deliberate tradeoff to make every host
> integration work from a container. If your threat model doesn't accept
> that, use bare-metal. See [`SECURITY.md`](./SECURITY.md).

---

## Path A — Bare-metal (Debian / Ubuntu / Raspberry Pi OS)

The most polished path, smallest blast radius. The installer:

- Creates a dedicated `controlroom` system user and group.
- Installs the binary to `/opt/controlroom/controlroom`.
- Creates the data directory `/var/lib/controlroom/` (mode 0750, owned by
  the controlroom user). TLS material, the JWT signing key, the SQLite DB
  and the audit log all live here.
- Drops `/etc/systemd/system/controlroom.service` — a hardened unit with
  `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, system-call
  filtering, and supplementary groups (`adm` for journal, `docker` if
  present).
- Drops `/etc/sudoers.d/controlroom` — a tight allowlist for the few
  commands ControlRoom needs to escalate (`apt-get update/install`,
  `systemctl reboot`, the specific `ufw` verbs the SPA exposes). Validated
  with `visudo -c` before install — the script aborts on parse failure.

### Prerequisites

- Debian 12+ / Ubuntu 22.04+ / Raspberry Pi OS Bookworm. Older releases
  haven't been tested.
- A user with `sudo` (only used for the install — ControlRoom doesn't run
  as root).
- Optional but expected for full feature parity: `ufw`, Docker (if you want
  the Containers tab), `kubectl` configured against your cluster (if you
  want the Kubernetes tab).

### Install

From a clone of the repo:

```bash
sudo deploy/install.sh
```

If you want to build from the cloned source rather than fetching a
pre-built binary, add `--from-source`:

```bash
sudo deploy/install.sh --from-source
```

The installer prints a one-time **setup token** at the end of its run and
also tails it from the journal. If you missed it, retrieve it with:

```bash
sudo journalctl -u controlroom -n 200 | grep setup_token
```

### First-run setup

1. Open `https://<host>:8443` in a browser. The first time, your browser
   will warn about the self-signed certificate — accept it for now, or
   switch to ACME / a reverse proxy later (see [TLS modes](#tls-modes)).
2. Paste the setup token in the wizard.
3. Pick an admin username + a strong password.
4. Optionally scan the QR code with Authy / 1Password / Aegis to enable
   TOTP. Verify the 6-digit code.
5. Done — you're logged in.

### Updating

```bash
cd /path/to/repo
git pull
sudo deploy/install.sh --from-source
```

The script preserves `/var/lib/controlroom/` (your data), restarts the
unit, and re-validates the sudoers fragment.

### Uninstall

```bash
sudo deploy/install.sh --uninstall
```

The data directory at `/var/lib/controlroom/` is *preserved* by default —
delete it manually for a truly fresh slate (you'll lose your account, JWT
key, TLS material, and audit log).

### Common pitfalls

- **`apt update` fails inside the apt job log**: the sudoers fragment
  whitelists `apt-get` not `apt`. The Go code calls `apt-get` for
  privileged ops; a bare `apt list --upgradable` runs unprivileged. If you
  see "sudo: a password is required", the fragment didn't install — check
  `sudo visudo -c -f /etc/sudoers.d/controlroom`.
- **Services tab empty**: the `controlroom` user needs to be in the `adm`
  group (for journal access) and able to talk to dbus. The installer does
  this automatically — verify with `id controlroom`.
- **Containers tab 503**: Docker daemon not present, or `controlroom` user
  not in the `docker` group. Add them: `sudo usermod -aG docker controlroom`,
  then `sudo systemctl restart controlroom`.

---

## Path B — Docker container (fat-privileged)

Single container that does everything. Drops distroless for `debian:bookworm-slim`
plus `bash`, `sudo`, `util-linux` (nsenter), `procps`, `iproute2`, `iptables`,
`ufw`, `systemd` (journalctl + systemctl + libsystemd0), `apt-utils`. Runs as
root with broad capabilities and host namespaces.

### Prerequisites

- Docker Engine 20.10+ (or compatible — Podman with the docker-compose plugin
  works too).
- The host's `docker` group GID (run `getent group docker | cut -d: -f3`).

If you want the Kubernetes tab to work in this shape, you also need:
- A reachable kubeconfig (`/etc/rancher/k3s/k3s.yaml` for K3s on the same
  host is the easiest case).
- Either `network_mode: host` (the default in our compose, so K3s'
  `127.0.0.1:6443` becomes reachable directly), **or** a host IP that's in
  your cluster's TLS SAN — see [Kubernetes integration](#kubernetes-integration).

### Install

```bash
git clone https://github.com/tm4rtin17/ControlRoom.git
cd ControlRoom

# Set DOCKER_GID for the compose file (only needed if you don't use
# network_mode:host or run as root — the default compose runs as root and
# doesn't need group_add, but it's still pulled from this var).
export DOCKER_GID=$(getent group docker | cut -d: -f3)

docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml logs --tail 200 | grep setup_token
```

### What the container can reach (and how)

The default `deploy/docker-compose.yml` mounts:

| Path | Mode | Used for |
|---|---|---|
| `/var/run/docker.sock` | ro | Containers tab |
| `/var/run/dbus/system_bus_socket` | rw | Services tab (systemctl via dbus) |
| `/run/log/journal` | ro | Logs tab (volatile journal) |
| `/var/log/journal` | ro | Logs tab (persistent journal, if enabled) |
| `/etc/machine-id` | ro | journalctl needs the host's machine-id to identify entries |
| `/etc/rancher/k3s/k3s.yaml` | ro | Kubernetes tab (mounted at `/etc/k3s/kubeconfig`) |
| `/var/cache/apt` | rw | Updates tab (apt cache; rw so `apt-get update` can refresh) |
| `/etc/apt` | ro | Updates tab (sources, preferences) |
| `/var/lib/dpkg` | ro | Updates tab (dpkg state) |
| `controlroom-data` | rw | App data (JWT key, SQLite, TLS) — Docker named volume |

Plus:
- `network_mode: host` so `ip`, `ufw`, and the K3s API at `127.0.0.1:6443`
  all see the host directly.
- `pid: host` so `nsenter -t 1 ...` finds the host's PID 1 (systemd).
- `cap_add: [SYS_ADMIN, SYS_PTRACE, NET_ADMIN]` for nsenter, host `ps`/`top`,
  and ufw/iptables.
- `security_opt: [apparmor:unconfined, seccomp:unconfined]` so the default
  Docker AppArmor and seccomp profiles don't block the host commands.

### Terminal: host login flow

The container sets `CR_HOST_SHELL=true` and `CR_TERMINAL_LOGIN=true` by
default. When you click the Terminal tab:

1. ControlRoom spawns `nsenter -t 1 -m -u -i -n -p -- /bin/bash -c <login script>`.
   This enters the host's mount, UTS, IPC, network, and PID namespaces.
2. The login script prints `ControlRoom Terminal — host login at <hostname>`
   and prompts for `username:`.
3. You type your host username, hit enter.
4. The script `exec`s `setpriv --reuid=nobody --regid=nogroup --clear-groups
   -- su -l <username>`. Dropping privileges first is critical: root
   invoking `su` skips PAM authentication ("root can become anyone").
5. `su` runs full PAM auth via `/etc/pam.d/su`, prompts for `Password:`,
   authenticates against `/etc/shadow`, logs the attempt to
   `/var/log/auth.log`, and on success drops to the user's shell at the
   user's uid.

Set `CR_TERMINAL_LOGIN=false` to skip the login flow (you land directly as
root inside the host namespaces — convenient but not recommended).

### Updating

```bash
git pull
docker compose -f deploy/docker-compose.yml up -d --build
```

For a published release tag, swap the `image:` in the compose file from
`controlroom:dev` (local build) to `ghcr.io/tm4rtin17/controlroom:v0.2.x`
and `docker compose pull && docker compose up -d`.

### Uninstall

```bash
docker compose -f deploy/docker-compose.yml down
docker volume rm controlroom-data    # nukes admin account, JWT key, TLS, audit log
```

### Common pitfalls

- **All tabs say "unavailable" on first load**: the boot log will show
  exactly which integration is failing. `docker compose logs controlroom`
  and look for `systemd unavailable`, `journalctl not in PATH`, etc. Each
  warning maps to a specific mount that didn't apply.
- **Terminal: "no usable shell on host"**: the image isn't the
  fat-privileged one (still distroless). Rebuild: `docker compose build`.
- **Terminal: drops you to root without password**: confirm
  `CR_TERMINAL_LOGIN=true` is set. `docker exec controlroom env | grep
  TERMINAL`. If it's set and you still skip the password prompt, check
  that `/usr/bin/setpriv` exists in the image: `docker exec controlroom
  which setpriv`.
- **Kubernetes tab hidden**: the host kubeconfig isn't mounted, or the
  network can't reach the API server. Verify the bind mount:
  `docker exec controlroom ls -la /etc/k3s/kubeconfig`. From the host:
  `curl -sk https://127.0.0.1:6443/livez` should return `ok`.
- **Logs tab works but Services doesn't**: the dbus socket isn't writable
  in the container, or the host doesn't run systemd (e.g. Alpine). Check
  `ls -la /var/run/dbus/system_bus_socket` on the host and `docker exec
  controlroom ls -la /var/run/dbus/system_bus_socket` in the container.

---

## Path C — In-cluster Kubernetes Pod

Run ControlRoom *inside* the cluster it manages. Useful when you want a
ControlRoom that's only for the K8s tab — the host integrations (Services,
Updates, Network, host Terminal, Journal Logs) are intentionally not wired
in this shape.

### Prerequisites

- A K3s / k8s cluster you can `kubectl apply` to.
- The `controlroom` image available to the cluster's runtime. Two options:
  1. Pull from a registry: edit `deploy/k8s/deployment.yaml` to point at
     `ghcr.io/tm4rtin17/controlroom:v0.2.x` and ensure your nodes can pull
     from GHCR.
  2. Side-load a locally-built image: `make image && docker save controlroom:dev
     | sudo k3s ctr images import -` (run on **every** node in a multi-node
     cluster — k3s' embedded containerd doesn't share images cluster-wide).
- Storage class. The PVC requests `local-path` (the K3s default). If your
  cluster uses a different storage class, edit `deploy/k8s/pvc.yaml`.
- An ingress controller if you want hostname-based access. The default
  `deploy/k8s/ingress.yaml` is a `networking.k8s.io/v1` Ingress with
  `ingressClassName: traefik` (K3s default).

### Install

```bash
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/rbac.yaml
kubectl apply -f deploy/k8s/pvc.yaml
kubectl apply -f deploy/k8s/deployment.yaml
kubectl apply -f deploy/k8s/service.yaml
# Optional:
kubectl apply -f deploy/k8s/ingress.yaml

kubectl -n controlroom rollout status deployment/controlroom --timeout=90s
kubectl -n controlroom logs deployment/controlroom | grep setup_token
```

The pod uses the projected ServiceAccount token to talk to the API server.
No kubeconfig is needed.

### What the ClusterRole grants

Phase A + B — strictly read-only. See `deploy/k8s/rbac.yaml` for the
canonical list:

```yaml
rules:
  - apiGroups: [""]
    resources: [nodes, namespaces, pods, services, configmaps, events, endpoints]
    verbs: [get, list, watch]
  - apiGroups: [""]
    resources: [pods/log]
    verbs: [get]
  - apiGroups: ["apps"]
    resources: [deployments, statefulsets, daemonsets, replicasets]
    verbs: [get, list, watch]
```

No `secrets`, no `persistentvolumes`, no write verbs. Phase C will widen
this to include `patch` and `delete` on Deployments / Pods for lifecycle
actions.

### Accessing the SPA

Three options, in order of typical preference:

1. **Ingress** (`deploy/k8s/ingress.yaml`): edit the host (defaults to
   `controlroom.roninlab.dev`), apply, point DNS at the ingress controller.
   For TLS, wire cert-manager separately — the manifest leaves it as a TODO
   comment.
2. **Port-forward** (works without ingress): `kubectl -n controlroom
   port-forward svc/controlroom 8443:443 --address 0.0.0.0` then visit
   `https://<your-machine>:8443`.
3. **NodePort** (edit the Service `spec.type` to `NodePort`): cluster-wide
   `:30443` or whatever NodePort gets assigned.

### First-run setup

Same wizard as the other shapes — paste the token, create your admin, etc.

### Updating

```bash
git pull
make image
docker save controlroom:dev | sudo k3s ctr images import -    # repeat on every node
kubectl -n controlroom rollout restart deployment/controlroom
kubectl -n controlroom rollout status deployment/controlroom --timeout=90s
```

### Uninstall

```bash
kubectl delete namespace controlroom    # nukes everything including the PVC
```

### Common pitfalls

- **`ImagePullBackOff` after install**: the image isn't on the node.
  `make image` only writes to your local Docker daemon, not k3s'
  containerd. Run the `docker save | k3s ctr images import` dance on every
  node.
- **`/api/k8s` returns 503 once you log in**: the projected SA token wasn't
  picked up. `kubectl -n controlroom describe pod -l app.kubernetes.io/name=controlroom`
  and check that `serviceAccountName: controlroom` is set on the spec.
- **Pod logs streaming fails with 403**: the ClusterRole is missing
  `pods/log`. Make sure you applied `rbac.yaml` from this branch (Phase A's
  rbac didn't include it; Phase B does).

---

## TLS modes

All three deployment shapes share the same TLS configuration. Set with
`CR_TLS_MODE`.

| Mode | Behaviour |
|---|---|
| `selfsigned` (default) | Generates an ECDSA P-256 self-signed cert at first boot under `$CR_DATA_DIR/tls/`. 10-year validity. SAN entries: `localhost`, the kernel hostname, every non-loopback IPv4 on the host. |
| `acme` | Let's Encrypt via HTTP-01 (`golang.org/x/crypto/acme/autocert`). Requires `CR_ACME_HOST` and `CR_ACME_EMAIL`. The host must be reachable on **port 80** from the public internet during issuance and renewal. Cache lives in `$CR_DATA_DIR/acme/`. |
| `proxy` | Bind plain HTTP and front with a TLS-terminating reverse proxy. Set `CR_ADDR=127.0.0.1:8080` (or similar) so only the proxy can reach the backend, and `CR_TRUST_PROXY=true` so `X-Forwarded-*` is honored for client IP / scheme. |

For container deployments using `acme`, you'll also need to add `- "80:80"`
to the `ports:` section (or remove `network_mode: host` and use port mapping).

For the in-cluster Pod, `acme` is not the right model — terminate TLS at
the ingress controller via cert-manager and run ControlRoom with
`CR_TLS_MODE=proxy` (or keep `selfsigned` and let the ingress proxy to
HTTPS).

---

## Kubernetes integration

ControlRoom's K8s client (`internal/k8s/client.go`) tries connection
sources in this order:

1. **In-cluster** (`rest.InClusterConfig()`) — uses the projected
   ServiceAccount token at `/var/run/secrets/kubernetes.io/serviceaccount/`.
   Used automatically when ControlRoom runs as a Pod with a SA mounted.
2. **`$KUBECONFIG`** — explicit override. Set this to point at any
   kubeconfig file readable by the controlroom process.
3. **`~/.kube/config`** — the standard local kubeconfig path.
4. **`/etc/rancher/k3s/k3s.yaml`** — K3s default location.

If none works, `/api/system/capabilities` reports `kubernetes: false` and
the SPA hides the Kubernetes tab.

### From a Docker container with a non-host network

If you run the container with a custom bridge instead of `network_mode: host`,
the kubeconfig's `server: https://127.0.0.1:6443` won't resolve to the host's
loopback. Use `deploy/scripts/setup-host-kubeconfig.sh` to generate a copy
with the server URL rewritten to a host IP that's in the cert SAN:

```bash
sudo deploy/scripts/setup-host-kubeconfig.sh
```

The script auto-detects the IP from `/etc/rancher/k3s/config.yaml` `node-ip`,
falling back to `hostname -I`. Override with `HOST_IP=`.

It writes `/var/lib/controlroom/kubeconfig` mode 0600 owned by 65532. Mount
that into the container at `/etc/k3s/kubeconfig` and set `KUBECONFIG=/etc/k3s/kubeconfig`.

---

## After install — checklist

1. **Read [`SECURITY.md`](./SECURITY.md)** for the threat model that matches
   your deployment shape.
2. **Lock down inbound access.** ControlRoom is not designed to be exposed
   directly to the public internet. Put it behind a VPN (Tailscale,
   WireGuard), an SSH tunnel, or a reverse proxy with strict firewall
   rules.
3. **Enable two-factor authentication** in Settings → Two-factor
   authentication. Required on every admin account in shared
   environments.
4. **Set strong host passwords** for any user the Terminal tab will let
   operators authenticate as. PAM is doing the work; weak host passwords
   = weak terminal access.
5. **Verify the audit log is writing**. Settings → Audit log. You should
   see your own login event from a moment ago.

## Next steps / further reading

- [`docs/CONFIG.md`](./CONFIG.md) — every environment variable and which
  tabs depend on each.
- [`docs/SECURITY.md`](./SECURITY.md) — full threat model, per-shape
  privilege scoping, what's enforced and what's a known gap.
- [`SPEC.md`](../SPEC.md) — design spec.
- [`MILESTONES.md`](../MILESTONES.md) — milestone tracker.
