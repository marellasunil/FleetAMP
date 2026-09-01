#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
SERVICE_NAME="fleetamp.service"
BINARY_PATH="${FLEETAMP_INSTALL_BIN:-$HOME/.local/bin/fleetamp}"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
UNIT_PATH="$UNIT_DIR/fleetamp.service"
OVERRIDE_DIR="$UNIT_DIR/fleetamp.service.d"
OVERRIDE_PATH="$OVERRIDE_DIR/10-local-paths.conf"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/fleetamp"
BACKUP_ROOT="$STATE_DIR/backups"
HEALTH_URL="${FLEETAMP_HEALTH_URL:-http://127.0.0.1:8080/health}"
HEALTH_TIMEOUT="${FLEETAMP_HEALTH_TIMEOUT:-30}"
ACTION="${1:-upgrade}"

log() { printf '[fleetamp-install] %s\n' "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

systemctl_user() {
  systemctl --user "$@"
}
wait_for_health() {
  local deadline=$((SECONDS + HEALTH_TIMEOUT))
  while (( SECONDS < deadline )); do
    if curl --fail --silent --show-error "$HEALTH_URL" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

restore_backup() {
  local backup_dir="$1"
  [[ -d "$backup_dir" ]] || fail "backup directory not found: $backup_dir"

  log "Restoring service files and binary from $backup_dir"
  systemctl_user stop "$SERVICE_NAME" >/dev/null 2>&1 || true
  [[ -f "$backup_dir/fleetamp" ]] &&
    install -D -m 0755 "$backup_dir/fleetamp" "$BINARY_PATH"
  [[ -f "$backup_dir/fleetamp.service" ]] &&
    install -D -m 0644 "$backup_dir/fleetamp.service" "$UNIT_PATH"
  [[ -f "$backup_dir/10-local-paths.conf" ]] &&
    install -D -m 0644 "$backup_dir/10-local-paths.conf" "$OVERRIDE_PATH"

  systemctl_user daemon-reload
  systemctl_user start "$SERVICE_NAME"
  wait_for_health || fail "restored service did not become healthy"
  log "Rollback completed and FleetAMP is healthy"
}
if [[ "$ACTION" == "rollback" ]]; then
  require_command systemctl
  require_command curl
  BACKUP_DIR="${2:-}"
  if [[ -z "$BACKUP_DIR" ]]; then
    BACKUP_DIR="$(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d 2>/dev/null |
      sort | tail -n 1)"
  fi
  [[ -n "$BACKUP_DIR" ]] || fail "no backup is available"
  restore_backup "$BACKUP_DIR"
  exit 0
fi

[[ "$ACTION" == "install" || "$ACTION" == "upgrade" ]] ||
  fail "usage: $0 [install|upgrade|rollback [backup-directory]]"

require_command go
require_command curl
require_command systemctl
require_command install
require_command tar

BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

log "Running Go tests"
(cd "$REPO_ROOT" && go test ./...)

log "Building FleetAMP"
(cd "$REPO_ROOT" && go build -trimpath -o "$BUILD_DIR/fleetamp" ./cmd/fleetamp)
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_DIR="$BACKUP_ROOT/$TIMESTAMP"
mkdir -p "$BACKUP_DIR" "$UNIT_DIR" "$OVERRIDE_DIR" "$STATE_DIR/log"

SERVICE_WAS_ACTIVE=false
if systemctl_user is-active --quiet "$SERVICE_NAME"; then
  SERVICE_WAS_ACTIVE=true
fi

log "Stopping FleetAMP for a consistent state backup"
systemctl_user stop "$SERVICE_NAME" >/dev/null 2>&1 || true

[[ -f "$BINARY_PATH" ]] && cp -a "$BINARY_PATH" "$BACKUP_DIR/fleetamp"
[[ -f "$UNIT_PATH" ]] && cp -a "$UNIT_PATH" "$BACKUP_DIR/fleetamp.service"
[[ -f "$OVERRIDE_PATH" ]] &&
  cp -a "$OVERRIDE_PATH" "$BACKUP_DIR/10-local-paths.conf"
if [[ -d "$REPO_ROOT/data" ]]; then
  tar -C "$REPO_ROOT" -czf "$BACKUP_DIR/data.tar.gz" data
fi

install -D -m 0755 "$BUILD_DIR/fleetamp" "$BINARY_PATH"
install -D -m 0644 "$REPO_ROOT/deploy/systemd/fleetamp-user.service" "$UNIT_PATH"
printf '%s\n' \
  '[Service]' \
  "WorkingDirectory=$REPO_ROOT" \
  "Environment=FLEETAMP_DATA_DIR=$REPO_ROOT/data" \
  "Environment=FLEETAMP_DATABASE_PATH=$REPO_ROOT/data/fleetamp.db" \
  >"$OVERRIDE_PATH"

systemctl_user daemon-reload
systemctl_user enable "$SERVICE_NAME" >/dev/null

log "Starting upgraded FleetAMP"
if ! systemctl_user start "$SERVICE_NAME" || ! wait_for_health; then
  log "Upgrade verification failed; restoring the previous installation"
  if [[ -f "$BACKUP_DIR/fleetamp" ]]; then
    restore_backup "$BACKUP_DIR"
  else
    systemctl_user stop "$SERVICE_NAME" >/dev/null 2>&1 || true
    fail "initial installation failed; no previous binary was available"
  fi
  exit 1
fi

log "FleetAMP is healthy at $HEALTH_URL"
log "Backup saved at $BACKUP_DIR"
if [[ "$SERVICE_WAS_ACTIVE" == false ]]; then
  log "The service was previously inactive and is now enabled and running"
fi
