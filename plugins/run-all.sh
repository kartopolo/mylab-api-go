#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Gateway port handling:
# - Default to :18081 for run-all to avoid clashing with existing dev gateway on :18080.
# - You can override via GATEWAY_HTTP_ADDR or HTTP_ADDR.
GATEWAY_HTTP_ADDR="${GATEWAY_HTTP_ADDR:-${HTTP_ADDR:-:18081}}"

is_port_in_use() {
  local addr="$1"
  local port
  port="${addr##*:}"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "(:|\])${port}$"
    return $?
  fi
  return 1
}

pick_gateway_addr() {
  local addr="$1"
  local port
  port="${addr##*:}"
  if [[ "$port" =~ ^[0-9]+$ ]] && is_port_in_use "$addr"; then
    local next=$((port+1))
    echo ":${next}"
    return 0
  fi
  echo "$addr"
}

GATEWAY_HTTP_ADDR="$(pick_gateway_addr "$GATEWAY_HTTP_ADDR")"

pids=()

cleanup() {
  # stop in reverse order
  for ((i=${#pids[@]}-1; i>=0; i--)); do
    pid="${pids[$i]}"
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
}

trap cleanup EXIT INT TERM

echo "[run-all] starting micro-dokter-go..."
"$ROOT_DIR/plugins/run-micro-dokter.sh" >/tmp/micro-dokter-go.log 2>&1 &
pids+=("$!")

# small delay so ports bind before health checks
sleep 0.2

echo "[run-all] starting micro-satusehat-go..."
"$ROOT_DIR/plugins/run-micro-satusehat.sh" >/tmp/micro-satusehat-go.log 2>&1 &
pids+=("$!")

sleep 0.2

echo "[run-all] starting gateway mylab-api-go on ${GATEWAY_HTTP_ADDR}..."
cd "$ROOT_DIR"

export HTTP_ADDR="$GATEWAY_HTTP_ADDR"
./bin/mylab-api-go
