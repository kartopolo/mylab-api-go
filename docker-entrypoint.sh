#!/bin/sh
set -eu

# Ensure writable session directory when using AUTH_SESSION_DRIVER=file.
# Named docker volumes are often root-owned on first create, so we chown once.

driver="${AUTH_SESSION_DRIVER:-file}"
files="${AUTH_SESSION_FILES:-/app/storage/sessions}"

case "$(echo "$driver" | tr '[:upper:]' '[:lower:]')" in
  file|"")
    mkdir -p "$files"
    # Best-effort: if running as root, ensure app user can write.
    # If chown fails (e.g., read-only mount), we still proceed and let app error clearly.
    chown -R app:app "$files" 2>/dev/null || true
    ;;
  *)
    ;;
esac

exec su-exec app:app /usr/local/bin/mylab-api-go
