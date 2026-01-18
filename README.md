# mylab-api-go

Go-based REST API layer for MyLab.

- Contract-first: see `Docs/openapi/openapi.yaml`
- Standard response envelope: `ok/message/errors`
- Transactional workflows with DB rollback on error

## Quick Start

### Configuration
```bash
# Copy environment template
cp .env.example .env

# Edit configuration (database, port, etc)
nano .env
```

### Service Status
```bash
# Cek status service
systemctl status mylab-api-go

# Test API
curl http://localhost:18080/healthz
```

### Deploy Changes (After Code Update)
```bash
# Quick deploy (build + restart)
./scripts/deploy.sh

# Or manual:
go build -o bin/mylab-api-go ./cmd/mylab-api-go
sudo systemctl restart mylab-api-go
```

### Development with Auto-Reload
```bash
# Watch for changes and auto-rebuild
./scripts/dev-watch.sh
```

### Access Info
- **Local**: `http://localhost:18080`
- **Port**: `18080`
- **Health Check**: `GET /healthz`
- **API Endpoints**: `/v1/*` (requires `X-User-Id` header, except `/v1/auth/login`)

## Testing & Debugging

### VS Code REST Client (Recommended)
1. Install extension: **REST Client** (`humao.rest-client`)
2. Open file: [`api-tests.http`](api-tests.http)
3. Click "Send Request" above any test
4. View response in split panel

### Browser Console (for quick tests)
```javascript
// Login
fetch('http://localhost:18080/v1/auth/login', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({email: 'admin@example.com', password: 'password123'})
}).then(r => r.json()).then(console.log)
```

📖 **Full Testing Guide**: See [Docs/TESTING.md](Docs/TESTING.md)

## Configuration

**Environment Variables**: Loaded from `.env` file or system environment.

```bash
# Database connection
DATABASE_URL=postgres://tiara:tiara@localhost:15432/mylab?sslmode=disable

# HTTP server
HTTP_ADDR=:18080

# Logging
LOG_LEVEL=info
```

📖 **Full Configuration Guide**: See [Docs/CONFIGURATION.md](Docs/CONFIGURATION.md)

## Documentation
