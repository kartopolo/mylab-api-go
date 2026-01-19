# Environment Configuration Guide

## Overview

MyLab API (Go) supports two methods for loading configuration:

1. **`.env` file** (recommended for development)
2. **System environment variables** (recommended for production/systemd)

**Priority**: System environment variables > `.env` file

## Configuration Sources (Current Setup)

### Development (Manual Run)
```bash
# Config loaded from: .env file
cd /var/www/mylab-api-go
./bin/mylab-api-go
```

### Production (Systemd Service)
```bash
# Config loaded from: /etc/systemd/system/mylab-api-go.service
systemctl status mylab-api-go
```

**Current source**: `/etc/systemd/system/mylab-api-go.service`
```ini
Environment=HTTP_ADDR=:18080
Environment=DATABASE_URL=postgres://tiara:tiara@localhost:15432/mylab?sslmode=disable
Environment=LOG_LEVEL=info
```

## Setup Instructions

### 1. Create `.env` File

```bash
# Copy from example
cd /var/www/mylab-api-go
cp .env.example .env

# Edit with your values
nano .env
```

### 2. Configure Database Connection

Edit `.env`:
```bash
# PostgreSQL (current setup)
DATABASE_URL=postgres://tiara:tiara@localhost:15432/mylab?sslmode=disable

# MySQL (alternative)
# DATABASE_URL=mysql://user:password@localhost:3306/mylab

# Remote PostgreSQL
# DATABASE_URL=postgres://user:pass@remote-host:5432/mylab?sslmode=require
```

### 3. Configure HTTP Server

```bash
# Listen on specific port
HTTP_ADDR=:18080

# Listen on specific IP
# HTTP_ADDR=192.168.1.100:18080

# Listen on all interfaces
# HTTP_ADDR=0.0.0.0:18080
```

### 4. Configure Logging

```bash
# Options: debug, info, warn, error
LOG_LEVEL=info

# Debug mode (verbose logging)
# LOG_LEVEL=debug

# Production (errors only)
# LOG_LEVEL=error
```

## Available Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HTTP_ADDR` | Yes | `:8080` | HTTP server listen address |
| `DATABASE_URL` | No | - | Database connection string |
| `LOG_LEVEL` | No | `info` | Logging level (debug/info/warn/error) |
| `ENVIRONMENT` | No | `development` | Environment name |
| `CORS_ALLOWED_ORIGINS` | No | localhost | Comma-separated allowed origins |
| `QUERYDSL_DENIED_TABLES` | No | - | Comma-separated denylist for `POST /v1/query` tables. If empty, all tables are allowed. Use `*` to deny all tables. |
| `CRUD_DENIED_TABLES` | No | - | Comma-separated denylist for `/v1/crud/{table}`. If empty, all tables are allowed. Use `*` to deny all tables. |
| `SCHEMA_DIR` | No | - | Directory containing `{table}.txt` schema files (externalized model). Used by schema-driven CRUD/services; falls back to DB introspection when missing. |
| `DB_SCHEMA` | No | `public` | Postgres schema name used for DB introspection (information_schema). |
| `PLUGIN_DIR` | No | - | Directory containing `*.json` plugin proxy configs. Enables routing under `/v1/plugins/*` to upstream microservices. |
| `RL_RATE_PER_MIN` | No | `60` | Rate limit: allowed requests per minute per IP for `/v1/crud/*`. |
| `RL_BURST` | No | `20` | Rate limit burst capacity (maximum tokens) per IP. |
| `AUTH_SESSION_DRIVER` | No | `file` | Auth session store driver for JWT sessions. Options: `file`, `postgres`/`database`, `none`. |
| `AUTH_SESSION_FILES` | No | `storage/sessions` | Directory for file-based auth sessions (default Laravel-like path). |
| `AUTH_SESSION_TABLE` | No | `auth_sessions` | Table name for Postgres-backed auth sessions. |

## Database Connection Formats

### PostgreSQL
```
postgres://username:password@host:port/database?sslmode=disable
```

**Examples:**
```bash
# Local PostgreSQL
postgres://tiara:tiara@localhost:5432/mylab?sslmode=disable

# Docker PostgreSQL
postgres://tiara:tiara@postgres:5432/mylab?sslmode=disable

# Remote PostgreSQL with SSL
postgres://user:pass@db.example.com:5432/mylab?sslmode=require

# Connection pooling
postgres://user:pass@localhost:5432/mylab?pool_max_conns=25
```

### MySQL (if needed)
```
mysql://username:password@tcp(host:port)/database?parseTime=true
```

## Load Priority

Configuration is loaded in this order (last wins):

1. **Built-in defaults** (hardcoded in `internal/config/config.go`)
2. **`.env` file** (if exists in project root)
3. **System environment variables** (always take precedence)

### Example Priority

```bash
# .env file
HTTP_ADDR=:8080

# System env (overrides .env)
export HTTP_ADDR=:18080

# Result: :18080 is used
```

## Usage Scenarios

### Scenario 1: Development (Use .env)

```bash
# 1. Create .env
cp .env.example .env

# 2. Edit values
nano .env

# 3. Run directly
./bin/mylab-api-go
```

**Benefit**: Easy to change config without restarting systemd.

### Scenario 2: Production (Use Systemd)

```bash
# 1. Edit service file
sudo nano /etc/systemd/system/mylab-api-go.service

# 2. Update environment variables
Environment=DATABASE_URL=postgres://...

# 3. Reload & restart
sudo systemctl daemon-reload
sudo systemctl restart mylab-api-go
```

**Benefit**: Centralized config, no .env file needed, secure.

### Scenario 3: Docker (Use .env)

```bash
# 1. Create .env
cp .env.example .env

# 2. Use in docker-compose.yml
env_file:
  - .env

# 3. Run
docker-compose up
```

### Scenario 5: Docker + Persistent Auth Sessions (Volume)

If you use `AUTH_SESSION_DRIVER=file` (default), mount a volume for `storage/sessions` so logout/session state survives container restarts.

Example snippet:

```yaml
services:
  mylab_api_go:
    environment:
      - AUTH_SESSION_DRIVER=file
      - AUTH_SESSION_FILES=/app/storage/sessions
    volumes:
      - mylab_api_sessions:/app/storage/sessions

volumes:
  mylab_api_sessions:
```

If you use `AUTH_SESSION_DRIVER=postgres`, the server auto-creates the `AUTH_SESSION_TABLE` (default `auth_sessions`) in the configured database.

### Scenario 4: Mixed (Systemd + .env Fallback)

```bash
# Service file only has critical configs
Environment=HTTP_ADDR=:18080

# .env has database and other configs
# (app will load .env if DATABASE_URL not in systemd)
```

## Security Best Practices

### 1. Never Commit .env to Git
```bash
# Already in .gitignore
.env
.env.local
.env.*.local
```

### 2. Use Different .env per Environment
```bash
.env.development   # Dev database
.env.staging       # Staging database
.env.production    # Production database (not in repo!)
```

### 3. Protect .env File Permissions
```bash
chmod 600 .env
chown root:root .env
```

### 4. Use Secrets Manager (Production)
Consider using:
- HashiCorp Vault
- AWS Secrets Manager
- Kubernetes Secrets
- Environment variables from CI/CD

## Updating Configuration

### Update .env File
```bash
# 1. Edit .env
nano /var/www/mylab-api-go/.env

# 2. Restart application
# If running via systemd:
sudo systemctl restart mylab-api-go

# If running manually:
# Just restart the process
```

### Update Systemd Service
```bash
# 1. Edit service file
sudo nano /etc/systemd/system/mylab-api-go.service

# 2. Reload daemon
sudo systemctl daemon-reload

# 3. Restart service
sudo systemctl restart mylab-api-go
```

## Troubleshooting

### Config Not Loaded
```bash
# Check if .env exists
ls -la /var/www/mylab-api-go/.env

# Check .env content
cat /var/www/mylab-api-go/.env

# Check systemd service config
systemctl cat mylab-api-go | grep Environment
```

### Database Connection Failed
```bash
# Test connection manually
psql "postgres://tiara:tiara@localhost:15432/mylab"

# Check DATABASE_URL format
echo $DATABASE_URL

# Check logs
sudo journalctl -u mylab-api-go -n 50
```

### Port Already in Use
```bash
# Check what's using the port
sudo lsof -i :18080

# Change port in .env or systemd
HTTP_ADDR=:18081
```

### Environment Variables Not Working
```bash
# Test if .env is loaded
cd /var/www/mylab-api-go
HTTP_ADDR=:9999 ./bin/mylab-api-go

# Should listen on :9999
# If not, check config.Load() implementation
```

## Migration from Old Setup

### Before (Hardcoded Config)
```go
// Old config
DATABASE_URL := "postgres://tiara:tiara@localhost:5432/mylab"
```

### After (Environment Config)
```bash
# .env file
DATABASE_URL=postgres://tiara:tiara@localhost:15432/mylab?sslmode=disable

# Or systemd
Environment=DATABASE_URL=postgres://...
```

**Benefits:**
- ✅ No code change needed to update config
- ✅ Different config per environment
- ✅ Secrets not in code
- ✅ Easy deployment

## Current Configuration Check

```bash
# Show current config source
systemctl show mylab-api-go | grep Environment

# Or check running process
ps aux | grep mylab-api-go
cat /proc/$(pgrep mylab-api-go)/environ | tr '\0' '\n'
```

## References

- [.env.example](../.env.example) - Template configuration
- [config.go](../internal/config/config.go) - Configuration loader implementation
- [systemd service](../scripts/mylab-api-go.service) - Service configuration
- [godotenv](https://github.com/joho/godotenv) - .env file loader library
