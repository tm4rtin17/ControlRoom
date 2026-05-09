# syntax=docker/dockerfile:1.6
#
# Multi-stage build for ControlRoom:
#   1. node-builder  — `vite build` → web/dist/
#   2. go-builder    — `go build` with embed.FS picking up web/dist
#   3. runtime       — debian:bookworm-slim, fat-privileged, root user, ~150-200 MB
#
# FAT-PRIVILEGED SHAPE
# ====================
# This image is designed to run with:
#   pid: host            — so nsenter -t 1 can reach systemd (PID 1 on the host)
#   cap_add: [SYS_ADMIN, SYS_PTRACE, NET_ADMIN]
#   security_opt: [apparmor:unconfined, seccomp:unconfined]
#   network_mode: host   — so `ip addr` and ufw see host interfaces
#   user: root (uid 0)   — required; Linux drops caps on uid transitions
#   mounts: see docker-compose.yml for the full list
#
# The distroless/nonroot shape has been intentionally dropped to enable
# Terminal (nsenter), Journal Logs (journalctl), Services (dbus),
# Updates (apt/systemctl), and Network (ip/ufw) integrations.
#
# Image size: ~150-200 MB (up from ~20 MB distroless). Expected and acceptable.
#
# UNPRIVILEGED PATH
# =================
# deploy/install.sh installs ControlRoom as a bare-metal systemd service with a
# scoped sudoers file. Use that if the privilege model of this container concerns
# you. The in-cluster K8s manifest (deploy/k8s/) also runs nonroot with a
# read-only ClusterRole.

# ---------- 1. frontend ----------
FROM node:20-alpine AS web-builder

WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY web/ ./
RUN npm run build

# ---------- 2. backend ----------
FROM golang:1.25-alpine AS go-builder

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

RUN apk add --no-cache ca-certificates git

# Pre-warm module cache before copying full source.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/web/dist ./internal/web/dist

# Pre-create the data dir so we can COPY it into the runtime stage.
RUN mkdir -p /out/data

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
      -X github.com/tm4rtin17/controlroom/internal/buildinfo.Version=${VERSION} \
      -X github.com/tm4rtin17/controlroom/internal/buildinfo.Commit=${COMMIT} \
      -X github.com/tm4rtin17/controlroom/internal/buildinfo.Date=${DATE}" \
    -o /out/controlroom ./cmd/controlroom

# ---------- 3. runtime ----------
FROM debian:bookworm-slim

# Install host-integration dependencies in a single layer; purge apt lists to
# keep the layer as small as possible.
#
#   bash         — host shell via nsenter + script use
#   ca-certificates  — TLS roots for outbound HTTPS
#   sudo         — wraps existing `sudo -n` calls in apt/ufw code (root → no-op,
#                  but the binary must be present)
#   util-linux   — provides nsenter for host-shell mode (Terminal tab)
#   procps       — ps/top inside the host PID namespace
#   iproute2     — `ip -j addr show` for the Network tab
#   iptables     — required by ufw
#   ufw          — host firewall control (Network tab)
#   systemd      — provides journalctl + systemctl + libsystemd0 (dbus client)
#   apt-utils    — apt helpers used by Updates tab
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      sudo \
      util-linux \
      procps \
      iproute2 \
      iptables \
      ufw \
      systemd \
      apt-utils \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /out/controlroom /app/controlroom
COPY --from=go-builder /out/data /data

ENV CR_DATA_DIR=/data \
    CR_ADDR=:8443

VOLUME ["/data"]
EXPOSE 8443

# Run as root. SYS_ADMIN and other broad caps are dropped by the kernel on any
# uid transition away from root, so we must stay uid 0.
# Security boundary is the host itself — see the comment at the top of this file.

ENTRYPOINT ["/app/controlroom"]
