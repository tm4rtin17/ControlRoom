#!/usr/bin/env bash
#
# NOTE: This script is NO LONGER NEEDED for the standard docker-compose
# deployment. Since docker-compose.yml now uses network_mode:host, the
# container shares the host network namespace and can reach the K3s API
# at 127.0.0.1:6443 directly. The original /etc/rancher/k3s/k3s.yaml is
# mounted read-only as /etc/k3s/kubeconfig inside the container — no
# server-URL rewrite is required.
#
# This script is retained for non-host-network deployments (e.g. a custom
# bridge network or a remote ControlRoom instance pointing at a separate
# K3s host). In those cases the container cannot reach 127.0.0.1:6443, so
# the server URL must be rewritten to the host's real IP that is in the
# K3s TLS SAN list.
#
# Usage (non-host-network deployments only):
#
#   sudo deploy/scripts/setup-host-kubeconfig.sh
#
# Override the host IP if the auto-detected one isn't in the cert SAN:
#
#   sudo HOST_IP=192.168.1.50 deploy/scripts/setup-host-kubeconfig.sh
#
# Note: the K3s admin kubeconfig contains cluster-admin credentials; the
# resulting file under /var/lib/controlroom/kubeconfig is mode 0600 and
# owned by uid 65532 (the controlroom container's nonroot user). Anyone
# with read access to that file has root on the cluster.

set -euo pipefail

K3S_CONFIG=${K3S_CONFIG:-/etc/rancher/k3s/k3s.yaml}
OUT_DIR=${OUT_DIR:-/var/lib/controlroom}
OUT=${OUT:-${OUT_DIR}/kubeconfig}
CR_UID=65532
CR_GID=65532

if [ ! -r "$K3S_CONFIG" ]; then
  echo "error: $K3S_CONFIG not readable — run as root or use sudo" >&2
  exit 1
fi

# Pick a host IP that's in K3s' cert SAN.
# Priority: $HOST_IP override → node-ip from /etc/rancher/k3s/config.yaml
# → first non-loopback IPv4 from `hostname -I`.
HOST_IP=${HOST_IP:-}
if [ -z "$HOST_IP" ] && [ -r /etc/rancher/k3s/config.yaml ]; then
  HOST_IP=$(awk '/^node-ip:/ {gsub(/[",]/,"",$2); print $2; exit}' /etc/rancher/k3s/config.yaml || true)
fi
if [ -z "$HOST_IP" ]; then
  HOST_IP=$(hostname -I | awk '{print $1}')
fi
if [ -z "$HOST_IP" ]; then
  echo "error: could not determine host IP — set HOST_IP=… and re-run" >&2
  exit 1
fi

# Verify the IP is in the cert SAN before we hand the container a config that
# will fail validation at runtime.
CERT=/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt
if [ -r "$CERT" ]; then
  if ! openssl x509 -in "$CERT" -noout -ext subjectAltName 2>/dev/null \
       | grep -q "IP Address:$HOST_IP\b"; then
    echo "warning: $HOST_IP not in cert SAN of $CERT" >&2
    echo "         the container will fail TLS verification — re-run with HOST_IP=… set to a SAN-correct address" >&2
  fi
fi

mkdir -p "$OUT_DIR"
sed "s|server: https://127.0.0.1:6443|server: https://${HOST_IP}:6443|" "$K3S_CONFIG" > "$OUT"
chown "$CR_UID:$CR_GID" "$OUT"
chmod 0600 "$OUT"

echo "wrote $OUT"
echo "  server: https://${HOST_IP}:6443"
echo "  owner:  ${CR_UID}:${CR_GID}"
echo "  mode:   0600"
