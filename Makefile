.PHONY: help build run test deploy restart status logs health clean

.DEFAULT_GOAL := help

help: ## Show this help
	@echo "MyLab API (Go) - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "🔨 Building..."
	go build -o bin/mylab-api-go ./cmd/mylab-api-go
	@echo "✅ Build complete: bin/mylab-api-go"

run: build ## Build and run locally (no systemd)
	@echo "🚀 Running locally on :18080..."
	HTTP_ADDR=:18080 \
	DATABASE_URL=postgres://tiara:tiara@localhost:15432/mylab?sslmode=disable \
	LOG_LEVEL=info \
	./bin/mylab-api-go

test: ## Run tests
	@echo "🧪 Running tests..."
	go test -v ./...

deploy: build ## Build and deploy (restart systemd service)
	@echo "♻️  Restarting service..."
	sudo systemctl restart mylab-api-go
	@sleep 1
	@echo "✅ Deployed!"
	@make health

restart: ## Restart systemd service
	@echo "♻️  Restarting service..."
	sudo systemctl restart mylab-api-go
	@sleep 1
	@make status

status: ## Show service status
	@systemctl status mylab-api-go --no-pager -l || true

logs: ## Show real-time logs
	@echo "📝 Showing logs (Ctrl+C to exit)..."
	sudo journalctl -u mylab-api-go -f

logs-err: ## Show error logs only
	@echo "❌ Error logs:"
	sudo journalctl -u mylab-api-go -p err -n 50 --no-pager

health: ## Check API health
	@echo "🏥 Health check:"
	@curl -sf http://localhost:18080/healthz | jq '.' 2>/dev/null || curl -s http://localhost:18080/healthz
	@echo ""

dev: ## Development mode with auto-reload
	@./scripts/dev-watch.sh

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	rm -f bin/mylab-api-go
	rm -f host-run.pid
	@echo "✅ Clean complete"

# Docker commands (jika diperlukan nanti)
docker-build: ## Build Docker image
	docker build -t mylab-api-go:latest .

docker-run: ## Run in Docker
	docker run -p 18080:8080 --rm mylab-api-go:latest
