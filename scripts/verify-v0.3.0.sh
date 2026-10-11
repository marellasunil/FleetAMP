#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION=${FLEETAMP_VERIFY_VERSION:-v0.3.0-rc1}
HTTP_ADDR=${FLEETAMP_VERIFY_HTTP_ADDR:-127.0.0.1:18082}
OPAMP_ADDR=${FLEETAMP_VERIFY_OPAMP_ADDR:-127.0.0.1:14322}
WITH_RACE=true
WITH_DOCKER=false

usage() {
  printf '%s\n' \
    "Usage: scripts/verify-v0.3.0.sh [--quick] [--docker]" \
    "" \
    "  --quick   Skip the race detector." \
    "  --docker  Build fleetamp:${VERSION} after binary verification." \
    "" \
    "Optional environment:" \
    "  FLEETAMP_VERIFY_EXISTING_DB=/path/to/fleetamp.db" \
    "  FLEETAMP_VERIFY_HTTP_ADDR=127.0.0.1:18082" \
    "  FLEETAMP_VERIFY_OPAMP_ADDR=127.0.0.1:14322" \
    "  FLEETAMP_VERIFY_VERSION=v0.3.0-rc1"
}

while (($#)); do
  case "$1" in
    --quick) WITH_RACE=false ;;
    --docker) WITH_DOCKER=true ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

log() { printf '[verify-v0.3.0] %s\n' "$*"; }
fail() { printf '[verify-v0.3.0] ERROR: %s\n' "$*" >&2; exit 1; }

command -v go >/dev/null || fail "Go is required"
command -v curl >/dev/null || fail "curl is required"

case "$HTTP_ADDR" in
  127.0.0.1:*|localhost:*) ;;
  *) fail "verification HTTP address must remain on loopback" ;;
esac
case "$OPAMP_ADDR" in
  127.0.0.1:*|localhost:*) ;;
  *) fail "verification OpAMP address must remain on loopback" ;;
esac

VERIFY_ROOT=$(mktemp -d)
DATA_DIR="$VERIFY_ROOT/data"
BINARY="$VERIFY_ROOT/fleetamp"
LOG_FILE="$VERIFY_ROOT/fleetamp.log"
PID=""

cleanup() {
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -rf "$VERIFY_ROOT"
}
trap cleanup EXIT INT TERM

wait_for_health() {
  local attempt
  for attempt in $(seq 1 60); do
    if curl --fail --silent "http://${HTTP_ADDR}/health" >/dev/null 2>&1; then
      return 0
    fi
    if [[ -n "$PID" ]] && ! kill -0 "$PID" 2>/dev/null; then
      return 1
    fi
    sleep 0.25
  done
  return 1
}

start_fleetamp() {
  : >"$LOG_FILE"
  FLEETAMP_HTTP_ADDR="$HTTP_ADDR" \
  FLEETAMP_OPAMP_ADDR="$OPAMP_ADDR" \
  FLEETAMP_DATA_DIR="$DATA_DIR" \
  FLEETAMP_DATABASE_PATH="$DATA_DIR/fleetamp.db" \
  FLEETAMP_LOG_FORMAT=json \
  FLEETAMP_LOG_FILE="$LOG_FILE" \
  FLEETAMP_GITOPS_WORKER_ENABLED=false \
  FLEETAMP_GITOPS_GITHUB_WRITES_ENABLED=false \
    "$BINARY" >"$VERIFY_ROOT/stdout.log" 2>"$VERIFY_ROOT/stderr.log" &
  PID=$!
  if ! wait_for_health; then
    sed -n '1,160p' "$VERIFY_ROOT/stderr.log" >&2 || true
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    fail "FleetAMP did not become healthy at http://${HTTP_ADDR}/health"
  fi
}

stop_fleetamp() {
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID"
    wait "$PID" || true
  fi
  PID=""
}

cd "$REPO_ROOT"
log "checking patch whitespace"
git diff --check

log "running unit and integration tests"
go test ./...

log "running go vet"
go vet ./...

if [[ "$WITH_RACE" == true ]]; then
  log "running race detector"
  go test -race ./...
fi

log "building ${VERSION} binary"
go build -buildvcs=false -trimpath \
  -ldflags="-s -w -X main.version=${VERSION}" \
  -o "$BINARY" ./cmd/fleetamp

mkdir -p "$DATA_DIR"
if [[ -n "${FLEETAMP_VERIFY_EXISTING_DB:-}" ]]; then
  [[ -f "$FLEETAMP_VERIFY_EXISTING_DB" ]] || fail "existing database does not exist: $FLEETAMP_VERIFY_EXISTING_DB"
  log "copying existing database fixture for upgrade verification"
  cp "$FLEETAMP_VERIFY_EXISTING_DB" "$DATA_DIR/fleetamp.db"
else
  log "using a fresh database; set FLEETAMP_VERIFY_EXISTING_DB to test a v0.2 copy"
fi

log "starting FleetAMP with Git writes disabled"
start_fleetamp
curl --fail --silent --show-error "http://${HTTP_ADDR}/ready" >/dev/null
[[ -s "$DATA_DIR/fleetamp.db" ]] || fail "FleetAMP database was not created"

log "restarting FleetAMP against the same database"
stop_fleetamp
start_fleetamp
curl --fail --silent --show-error "http://${HTTP_ADDR}/health" >/dev/null
curl --fail --silent --show-error "http://${HTTP_ADDR}/ready" >/dev/null
stop_fleetamp

if [[ "$WITH_DOCKER" == true ]]; then
  command -v docker >/dev/null || fail "Docker is required for --docker"
  docker info >/dev/null 2>&1 || fail "Docker daemon is not available"
  log "building Docker image fleetamp:${VERSION}"
  docker build --build-arg VERSION="$VERSION" -t "fleetamp:${VERSION}" .
fi

log "PASS: tests, vet, binary build, health, readiness, database migration and restart"
