#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

pids=()

cleanup() {
  for ((i=${#pids[@]}-1; i>=0; i--)); do
    pid="${pids[$i]}"
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
}

trap cleanup EXIT INT TERM

echo "[run-micros] starting micro-dokter-go..."
"$ROOT_DIR/plugins/run-micro-dokter.sh" >/tmp/micro-dokter-go.log 2>&1 &
pids+=("$!")

sleep 0.2

echo "[run-micros] starting micro-satusehat-go..."
"$ROOT_DIR/plugins/run-micro-satusehat.sh" >/tmp/micro-satusehat-go.log 2>&1 &
pids+=("$!")

echo "[run-micros] running. logs: /tmp/micro-dokter-go.log /tmp/micro-satusehat-go.log"

echo "[run-micros] press Ctrl+C to stop"

# wait forever (until Ctrl+C)
while true; do
  sleep 1
done
