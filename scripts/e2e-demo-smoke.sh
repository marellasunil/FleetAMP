#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${FLEETAMP_BASE_URL:-http://127.0.0.1:18080}"
ADMIN_USER="${FLEETAMP_ADMIN_USERNAME:-}"
ADMIN_PASSWORD="${FLEETAMP_ADMIN_PASSWORD:-}"

if [[ -z "$ADMIN_USER" || -z "$ADMIN_PASSWORD" ]]; then
  echo "Set FLEETAMP_ADMIN_USERNAME and FLEETAMP_ADMIN_PASSWORD." >&2
  exit 2
fi

WORK_DIR="$(mktemp -d)"
COOKIE_JAR="$WORK_DIR/admin.cookies"
BODY_FILE="$WORK_DIR/response.body"
trap 'rm -rf "$WORK_DIR"' EXIT

pass_count=0

pass() {
  pass_count=$((pass_count + 1))
  printf 'ok %d - %s\n' "$pass_count" "$1"
}

fail() {
  printf 'not ok %d - %s\n' "$((pass_count + 1))" "$1" >&2
  if [[ -s "$BODY_FILE" ]]; then
    sed -n '1,20p' "$BODY_FILE" >&2
  fi
  exit 1
}

request() {
  local method="$1"
  local path="$2"
  shift 2
  curl --silent --show-error --output "$BODY_FILE" --write-out '%{http_code}' \
    --request "$method" --cookie "$COOKIE_JAR" --cookie-jar "$COOKIE_JAR" \
    "$@" "$BASE_URL$path"
}

assert_page() {
  local path="$1"
  local marker="$2"
  local status
  status="$(request GET "$path")"
  [[ "$status" == "200" ]] || fail "GET $path returned HTTP $status"
  grep -Fq "$marker" "$BODY_FILE" || fail "GET $path is missing marker: $marker"
  pass "GET $path contains $marker"
}

printf '1..18\n'

status="$(curl --silent --show-error --output "$BODY_FILE" --write-out '%{http_code}' "$BASE_URL/health")"
[[ "$status" == "200" ]] && grep -Fq '"status":"ok"' "$BODY_FILE" || fail "health endpoint"
pass "health endpoint is ready"

status="$(request POST /login \
  --header "Origin: $BASE_URL" \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode "username=$ADMIN_USER" \
  --data-urlencode "password=$ADMIN_PASSWORD")"
[[ "$status" == "303" ]] || fail "administrator login returned HTTP $status"
pass "administrator login creates a session"

status="$(request GET /api/v1/session)"
[[ "$status" == "200" ]] && grep -Fq '"role":"admin"' "$BODY_FILE" || fail "admin session identity"
pass "session reports the Admin role"

assert_page /agents "Managed agents"
assert_page /groups "Agent groups"
assert_page /deployments "Deployments"
assert_page /approvals "Deployment approvals"
assert_page /instrumentation "Instrumentation Guides"
assert_page /blueprints "Telemetry Blueprints"
assert_page /ai-insights "AI Insights"
assert_page /mcp "MCP Server"
assert_page /pipelines "Pipelines"
assert_page /audit-log "Audit log"
assert_page /settings/users "Users and roles"
assert_page /settings/configuration-sections "Configuration section policies"
assert_page /settings/configuration-drift "Configuration drift"

status="$(request POST /api/v1/configurations/validate \
  --header "Origin: $BASE_URL" \
  --header 'Content-Type: application/json' \
  --data '{"content":"receivers:\n  otlp:\nservice:\n  pipelines: {}\n"}')"
[[ "$status" == "200" ]] && grep -Fq '"valid":true' "$BODY_FILE" || fail "configuration validation"
pass "valid Collector YAML passes validation"

status="$(request POST /api/v1/configurations/validate \
  --header "Origin: $BASE_URL" \
  --header 'Content-Type: application/json' \
  --data '{"content":"service:\n  pipelines: [\n"}')"
[[ "$status" == "422" ]] && grep -Fq '"valid":false' "$BODY_FILE" || fail "invalid configuration rejection"
pass "invalid Collector YAML is rejected"

printf '# FleetAMP smoke test passed: %d checks\n' "$pass_count"
