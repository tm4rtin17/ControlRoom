#!/usr/bin/env bash
# ControlRoom bare-metal installer for Debian / Ubuntu.
#
# Usage (from a clone):
#   sudo deploy/install.sh                   # install latest tarball release
#   sudo deploy/install.sh --from-source     # build the binary locally first
#   sudo deploy/install.sh --uninstall       # reverse a previous install
#
# Usage (one-liner):
#   curl -fsSL https://raw.githubusercontent.com/<owner>/controlroom/main/deploy/install.sh | sudo bash
#
# What it does:
#   1. Verifies we're on Debian or Ubuntu and have a few prerequisite tools.
#   2. Creates the controlroom system user + /etc/controlroom + /var/lib/controlroom + /opt/controlroom.
#   3. Downloads the release binary for the host architecture (or builds it).
#   4. Installs the systemd unit and sudoers fragment, validating the latter.
#   5. Adds controlroom to the docker group only if /var/run/docker.sock exists.
#   6. systemctl enable --now controlroom.
#   7. Tails the journal to surface the first-run setup token.

set -euo pipefail

readonly SERVICE="controlroom"
readonly USER_NAME="controlroom"
readonly INSTALL_DIR="/opt/controlroom"
readonly DATA_DIR="/var/lib/controlroom"
readonly CONFIG_DIR="/etc/controlroom"
readonly UNIT_PATH="/etc/systemd/system/${SERVICE}.service"
readonly SUDOERS_PATH="/etc/sudoers.d/controlroom"
readonly RELEASE_REPO="${CR_RELEASE_REPO:-tm4rtin17/ControlRoom}"

cmd_exists() { command -v "$1" >/dev/null 2>&1; }

log()  { printf "\033[1;36m[controlroom]\033[0m %s\n" "$*"; }
warn() { printf "\033[1;33m[controlroom]\033[0m %s\n" "$*" >&2; }
die()  { printf "\033[1;31m[controlroom]\033[0m %s\n" "$*" >&2; exit 1; }

require_root() {
  [[ $EUID -eq 0 ]] || die "must run as root (try: sudo $0 $*)"
}

check_distro() {
  if [[ ! -f /etc/os-release ]]; then
    die "/etc/os-release missing — unsupported distro"
  fi
  # shellcheck disable=SC1091
  . /etc/os-release
  case "${ID:-}" in
    debian|ubuntu) ;;
    *)
      case "${ID_LIKE:-}" in
        *debian*) ;;
        *) die "ControlRoom v0.1 supports Debian and Ubuntu only (detected: ${ID:-unknown})" ;;
      esac
      ;;
  esac
}

check_prereqs() {
  for c in systemctl visudo curl tar; do
    cmd_exists "$c" || die "missing required tool: $c"
  done
}

create_user() {
  if id "$USER_NAME" >/dev/null 2>&1; then
    log "user $USER_NAME already exists"
    return
  fi
  log "creating system user $USER_NAME"
  useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$USER_NAME"
}

ensure_dirs() {
  log "ensuring directories"
  install -d -m 0755 -o root -g root "$INSTALL_DIR"
  install -d -m 0750 -o "$USER_NAME" -g "$USER_NAME" "$DATA_DIR"
  install -d -m 0755 -o root -g root "$CONFIG_DIR"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

download_binary() {
  local arch dest
  arch=$(detect_arch)
  dest="$INSTALL_DIR/controlroom"
  log "downloading controlroom-linux-${arch} from $RELEASE_REPO"

  local url
  url="https://github.com/${RELEASE_REPO}/releases/latest/download/controlroom-linux-${arch}.tar.gz"
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  curl -fsSL "$url" -o "$tmp/release.tar.gz" || die "download failed: $url"
  tar -xzf "$tmp/release.tar.gz" -C "$tmp"
  install -m 0755 -o root -g root "$tmp/controlroom" "$dest"
}

build_from_source() {
  cmd_exists go || die "--from-source needs Go on PATH"
  cmd_exists make || die "--from-source needs make on PATH"
  cmd_exists npm || die "--from-source needs npm on PATH"
  log "building from source via make"
  make build
  install -m 0755 -o root -g root "controlroom" "$INSTALL_DIR/controlroom"
}

install_unit() {
  log "installing systemd unit"
  install -m 0644 -o root -g root "deploy/controlroom.service" "$UNIT_PATH" 2>/dev/null \
    || install_embedded_unit
  systemctl daemon-reload
}

# install_embedded_unit handles the curl|bash case where there's no checkout.
install_embedded_unit() {
  cat > "$UNIT_PATH" <<'EOF'
[Unit]
Description=ControlRoom — homelab dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=controlroom
Group=controlroom
EnvironmentFile=-/etc/controlroom/controlroom.env
WorkingDirectory=/var/lib/controlroom
ExecStart=/opt/controlroom/controlroom
Restart=on-failure
RestartSec=5

NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/controlroom
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
RestrictNamespaces=yes
LockPersonality=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service
SystemCallFilter=~@privileged @resources
SupplementaryGroups=adm docker

[Install]
WantedBy=multi-user.target
EOF
  chmod 0644 "$UNIT_PATH"
}

install_sudoers() {
  log "installing sudoers fragment"
  local src=deploy/controlroom.sudoers
  local tmp
  tmp="$(mktemp)"
  if [[ -f "$src" ]]; then
    cp "$src" "$tmp"
  else
    cat > "$tmp" <<'EOF'
Cmnd_Alias CR_APT = /usr/bin/apt-get update, \
                    /usr/bin/apt-get -y -o Dpkg\:\:Options\:\:=--force-confold upgrade
Cmnd_Alias CR_REBOOT = /bin/systemctl reboot, /usr/bin/systemctl reboot
Cmnd_Alias CR_UFW = /usr/sbin/ufw status numbered, \
                    /usr/sbin/ufw allow *, \
                    /usr/sbin/ufw deny *, \
                    /usr/sbin/ufw reject *, \
                    /usr/sbin/ufw limit *, \
                    /usr/sbin/ufw --force delete *, \
                    /usr/sbin/ufw --force enable, \
                    /usr/sbin/ufw disable
controlroom ALL=(root) NOPASSWD: CR_APT, CR_REBOOT, CR_UFW
Defaults:controlroom !requiretty
EOF
  fi
  visudo -cf "$tmp" >/dev/null || { rm -f "$tmp"; die "sudoers fragment is invalid"; }
  install -m 0440 -o root -g root "$tmp" "$SUDOERS_PATH"
  rm -f "$tmp"
}

maybe_join_docker_group() {
  if [[ -S /var/run/docker.sock ]]; then
    log "joining controlroom to the docker group (docker socket detected)"
    if ! getent group docker >/dev/null; then
      groupadd docker
    fi
    usermod -aG docker "$USER_NAME"
  fi
}

start_service() {
  log "enabling and starting $SERVICE"
  systemctl enable --now "$SERVICE"
}

print_setup_token() {
  log "waiting for first-run setup token (10s)…"
  if journalctl -u "$SERVICE" -n 200 --no-pager 2>/dev/null | grep -F "setup_token" | tail -n1; then
    return
  fi
  warn "no setup token in the journal yet. Run: journalctl -u $SERVICE -n 200 | grep setup_token"
}

uninstall() {
  log "stopping and disabling $SERVICE (errors below are OK if it isn't installed)"
  systemctl disable --now "$SERVICE" 2>/dev/null || true
  rm -f "$UNIT_PATH" "$SUDOERS_PATH"
  systemctl daemon-reload
  if [[ -d "$INSTALL_DIR" ]]; then
    rm -rf "$INSTALL_DIR"
  fi
  log "preserving $DATA_DIR (delete it manually if you want a fresh install)"
  if id "$USER_NAME" >/dev/null 2>&1; then
    log "user $USER_NAME left in place; remove with: userdel $USER_NAME"
  fi
  log "uninstalled."
}

main() {
  case "${1:-}" in
    --uninstall)
      require_root "$@"
      uninstall
      return
      ;;
  esac

  require_root "$@"
  check_distro
  check_prereqs

  create_user
  ensure_dirs

  case "${1:-}" in
    --from-source) build_from_source ;;
    *)             download_binary  ;;
  esac

  install_unit
  install_sudoers
  maybe_join_docker_group
  start_service
  print_setup_token

  log "done. Open https://$(hostname -f):8443"
}

main "$@"
