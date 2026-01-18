#!/usr/bin/env bash
set -euo pipefail

# Defaults for host-run
HTTP_ADDR=${HTTP_ADDR:-:18080}
DATABASE_URL=${DATABASE_URL:-postgres://tiara:tiara@localhost:15432/mylab?sslmode=disable}
LOG_LEVEL=${LOG_LEVEL:-info}

# Project root (scripts/ -> repo root)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="${SCRIPT_DIR%/scripts}"
BIN="$REPO_ROOT/bin/mylab-api-go"

cd "$REPO_ROOT"
mkdir -p bin

# Build if binary missing
if [[ ! -x "$BIN" ]]; then
  echo "Building mylab-api-go binary..."
  GOFLAGS='' go build -o "$BIN" ./cmd/mylab-api-go
fi

# Start in background with nohup
nohup env HTTP_ADDR="$HTTP_ADDR" DATABASE_URL="$DATABASE_URL" LOG_LEVEL="$LOG_LEVEL" "$BIN" > host-run.log 2>&1 &
PID=$!
echo "$PID" > host-run.pid
sleep 1

echo "Started mylab-api-go (PID $PID) on $HTTP_ADDR"
head -n 3 host-run.log || true
