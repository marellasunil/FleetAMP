#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION=${OTELPLANE_VERIFY_VERSION:-v0.3.0-rc1}
HTTP_ADDR=${OTELPLANE_VERIFY_HTTP_ADDR:-127.0.0.1:18082}
OPAMP_ADDR=${OTELPLANE_VERIFY_OPAMP_ADDR:-127.0.0.1:14322}
WITH_RACE=true
WITH_DOCKER=false

usage() {
  printf '%s\n' \
    "Usage: scripts/verify-v0.3.0.sh [--quick] [--docker]" \
    "" \
    "  --quick   Skip the race detector." \
    "  --docker  Build otelplane:${VERSION} after binary verification." \
    "" \
    "Optional environment:" \
    "  OTELPLANE_VERIFY_EXISTING_DB=/path/to/otelplane.db" \
    "  OTELPLANE_VERIFY_HTTP_ADDR=127.0.0.1:18082" \
    "  OTELPLANE_VERIFY_OPAMP_ADDR=127.0.0.1:14322" \
    "  OTELPLANE_VERIFY_VERSION=v0.3.0-rc1"
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
BINARY="$VERIFY_ROOT/otelplane"
LOG_FILE="$VERIFY_ROOT/otelplane.log"
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

start_otelplane() {
  : >"$LOG_FILE"
  OTELPLANE_HTTP_ADDR="$HTTP_ADDR" \
  OTELPLANE_OPAMP_ADDR="$OPAMP_ADDR" \
  OTELPLANE_DATA_DIR="$DATA_DIR" \
  OTELPLANE_DATABASE_PATH="$DATA_DIR/otelplane.db" \
  OTELPLANE_LOG_FORMAT=json \
  OTELPLANE_LOG_FILE="$LOG_FILE" \
  OTELPLANE_GITOPS_WORKER_ENABLED=false \
  OTELPLANE_GITOPS_GITHUB_WRITES_ENABLED=false \
    "$BINARY" >"$VERIFY_ROOT/stdout.log" 2>"$VERIFY_ROOT/stderr.log" &
  PID=$!
  if ! wait_for_health; then
    sed -n '1,160p' "$VERIFY_ROOT/stderr.log" >&2 || true
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    fail "OTelPlane did not become healthy at http://${HTTP_ADDR}/health"
  fi
}

stop_otelplane() {
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
  -o "$BINARY" ./cmd/otelplane

mkdir -p "$DATA_DIR"
if [[ -n "${OTELPLANE_VERIFY_EXISTING_DB:-}" ]]; then
  [[ -f "$OTELPLANE_VERIFY_EXISTING_DB" ]] || fail "existing database does not exist: $OTELPLANE_VERIFY_EXISTING_DB"
  log "copying existing database fixture for upgrade verification"
  cp "$OTELPLANE_VERIFY_EXISTING_DB" "$DATA_DIR/otelplane.db"
else
  log "using a fresh database; set OTELPLANE_VERIFY_EXISTING_DB to test a v0.2 copy"
fi

log "starting OTelPlane with Git writes disabled"
start_otelplane
curl --fail --silent --show-error "http://${HTTP_ADDR}/ready" >/dev/null
[[ -s "$DATA_DIR/otelplane.db" ]] || fail "OTelPlane database was not created"

log "restarting OTelPlane against the same database"
stop_otelplane
start_otelplane
curl --fail --silent --show-error "http://${HTTP_ADDR}/health" >/dev/null
curl --fail --silent --show-error "http://${HTTP_ADDR}/ready" >/dev/null
stop_otelplane

if [[ "$WITH_DOCKER" == true ]]; then
  command -v docker >/dev/null || fail "Docker is required for --docker"
  docker info >/dev/null 2>&1 || fail "Docker daemon is not available"
  log "building Docker image otelplane:${VERSION}"
  docker build --build-arg VERSION="$VERSION" -t "otelplane:${VERSION}" .
fi

log "PASS: tests, vet, binary build, health, readiness, database migration and restart"
