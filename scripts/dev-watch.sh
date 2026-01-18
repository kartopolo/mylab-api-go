#!/bin/bash
# MyLab API (Go) - Development Watch Script
# Auto-rebuild and restart on file changes
# Usage: ./scripts/dev-watch.sh

set -e

echo "👀 Watching for changes in ./internal and ./cmd..."
echo "Press Ctrl+C to stop"

# Initial build
cd /var/www/mylab-api-go
go build -o bin/mylab-api-go ./cmd/mylab-api-go
sudo systemctl restart mylab-api-go
echo "✅ Initial build complete"

# Watch for changes
while true; do
    # Wait for file changes
    inotifywait -qq -r -e modify,create,delete \
        --exclude '(\.git|bin|tmp|\.swp|~)' \
        ./internal ./cmd 2>/dev/null || {
        echo "⚠️  inotifywait not found. Install: sudo apt install inotify-tools"
        exit 1
    }
    
    echo ""
    echo "🔄 Changes detected, rebuilding..."
    
    if go build -o bin/mylab-api-go ./cmd/mylab-api-go; then
        sudo systemctl restart mylab-api-go
        echo "✅ Build successful, service restarted"
        curl -s http://localhost:18080/healthz > /dev/null && echo "✅ Service healthy"
    else
        echo "❌ Build failed, service not restarted"
    fi
    
    echo "👀 Watching for changes..."
done
