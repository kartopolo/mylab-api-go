#!/usr/bin/env bash
set -euo pipefail

# Defaults for host-run
HTTP_ADDR=${HTTP_ADDR:-:18080}
DATABASE_URL=${DATABASE_URL:-postgres://tiara:tiara@localhost:15432/mylab?sslmode=disable}
LOG_LEVEL=${LOG_LEVEL:-info}

# Laravel-like auth session storage (optional overrides).
AUTH_SESSION_DRIVER=${AUTH_SESSION_DRIVER:-file}
AUTH_SESSION_FILES=${AUTH_SESSION_FILES:-/var/www/mylab-api-go/storage/sessions}

# Project root (scripts/ -> repo root)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="${SCRIPT_DIR%/scripts}"
BIN="$REPO_ROOT/bin/mylab-api-go"

cd "$REPO_ROOT"
mkdir -p bin

# Stop previous run (best-effort)
if [[ -f "$REPO_ROOT/host-run.pid" ]]; then
  OLD_PID="$(cat "$REPO_ROOT/host-run.pid" 2>/dev/null || true)"
  if [[ -n "${OLD_PID}" ]] && kill -0 "${OLD_PID}" 2>/dev/null; then
    echo "Stopping previous mylab-api-go (PID ${OLD_PID})..."
    kill "${OLD_PID}" 2>/dev/null || true
    sleep 1
  fi
fi

# Build if binary missing or stale (or forced)
NEED_BUILD=0
if [[ ! -x "$BIN" ]]; then
  NEED_BUILD=1
fi
if [[ "${FORCE_REBUILD:-0}" == "1" ]]; then
  NEED_BUILD=1
fi
if [[ $NEED_BUILD -eq 0 ]]; then
  if find "$REPO_ROOT" -type f -name '*.go' \( -path "$REPO_ROOT/cmd/*" -o -path "$REPO_ROOT/internal/*" \) -newer "$BIN" | head -n 1 | grep -q .; then
    NEED_BUILD=1
  fi
fi

if [[ $NEED_BUILD -eq 1 ]]; then
  echo "Building mylab-api-go binary..."
  GOFLAGS='' go build -o "$BIN" ./cmd/mylab-api-go
fi

# Start in background with nohup
nohup env \
  HTTP_ADDR="$HTTP_ADDR" \
  DATABASE_URL="$DATABASE_URL" \
  LOG_LEVEL="$LOG_LEVEL" \
  AUTH_SESSION_DRIVER="$AUTH_SESSION_DRIVER" \
  AUTH_SESSION_FILES="$AUTH_SESSION_FILES" \
  "$BIN" > host-run.log 2>&1 &
PID=$!
echo "$PID" > host-run.pid
sleep 1

echo "Started mylab-api-go (PID $PID) on $HTTP_ADDR"
head -n 3 host-run.log || true
