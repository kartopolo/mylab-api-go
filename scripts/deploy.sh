#!/bin/bash
# MyLab API (Go) - Quick Deploy Script
# Usage: ./scripts/deploy.sh

set -e

echo "🔨 Building mylab-api-go..."
cd /var/www/mylab-api-go
go build -o bin/mylab-api-go ./cmd/mylab-api-go

echo "♻️  Restarting service..."
sudo systemctl restart mylab-api-go

echo "⏳ Waiting for service to start..."
sleep 2

echo "📊 Service status:"
systemctl status mylab-api-go --no-pager -l

echo ""
echo "✅ Health check:"
curl -s http://localhost:18080/healthz | jq '.' || curl -s http://localhost:18080/healthz

echo ""
echo "🎉 Deploy completed!"
echo "📝 View logs: sudo journalctl -u mylab-api-go -f"
