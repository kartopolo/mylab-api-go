# MyLab API (Go) - Deployment Guide

## Status Service

### Cek Status
```bash
# Cek apakah service sedang jalan
systemctl status mylab-api-go

# Cek port yang listen
netstat -tlnp | grep 18080
# atau
ss -tlnp | grep 18080

# Test health check
curl http://localhost:18080/healthz
```

## Setiap Perubahan Code - Workflow

### 1. **Build Binary Baru**
Setiap perubahan code **HARUS di-rebuild** karena Go adalah compiled language:
```bash
cd /var/www/mylab-api-go
go build -o bin/mylab-api-go ./cmd/mylab-api-go
```

### 2. **Restart Service**
Setelah build, restart service untuk load binary baru:
```bash
sudo systemctl restart mylab-api-go

# Cek status setelah restart
sudo systemctl status mylab-api-go

# Lihat log jika ada error
sudo journalctl -u mylab-api-go -f
```

### 3. **One-Liner Deploy** (Recommended)
```bash
cd /var/www/mylab-api-go && \
  go build -o bin/mylab-api-go ./cmd/mylab-api-go && \
  sudo systemctl restart mylab-api-go && \
  sleep 1 && \
  systemctl status mylab-api-go
```

## Service Configuration

**Location**: `/etc/systemd/system/mylab-api-go.service`

**Current Settings**:
- Port: `18080`
- Database: `postgres://tiara:tiara@localhost:15432/mylab`
- Log Level: `info`
- Auto-restart: `always` (restart every 2 seconds on failure)

### Update Service Configuration
Jika perlu ubah config (port, database, dll):
```bash
# 1. Edit service file
sudo nano /etc/systemd/system/mylab-api-go.service

# 2. Reload systemd
sudo systemctl daemon-reload

# 3. Restart service
sudo systemctl restart mylab-api-go
```

## Client Access

### Endpoints Available
```bash
# Health check (no auth)
GET http://localhost:18080/healthz

# Ready check (no auth)
GET http://localhost:18080/readyz

# Metrics (no auth)
GET http://localhost:18080/metrics

# Login (no auth)
POST http://localhost:18080/v1/auth/login
Content-Type: application/json
{
  "email": "user@example.com",
  "password": "password"
}

# All other /v1/* endpoints require authentication
# Header: Authorization: Bearer <JWT token>

## Docker Compose Ports

Jika menjalankan via docker compose di `/home/mylabapp/dockerdata`, port host default:
- API: `http://localhost:58080` (host) -> `:8080` (container)
```

### Akses dari External Client

#### 1. **Local Network** (WSL → Windows Host):
```bash
# WSL IP address
ip addr show eth0 | grep "inet\b" | awk '{print $2}' | cut -d/ -f1
# Example: 172.x.x.x

# Client akses dari:
http://172.x.x.x:18080/healthz
```

#### 2. **Internet Access** (jika ada public IP):
```bash
# Cek firewall
sudo ufw status

# Allow port 18080 (sudah dilakukan)
sudo ufw allow 18080/tcp
sudo ufw allow 18080/udp

# Client akses dari:
http://<your-public-ip>:18080/healthz
```

#### 3. **Via Nginx Reverse Proxy** (recommended untuk production):
```nginx
# /etc/nginx/sites-available/mylab-api
server {
    listen 80;
    server_name api.mylab.com;

    location / {
        proxy_pass http://localhost:18080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## Development Workflow

### Hot Reload (Not Supported by Default)
Go tidak support hot reload seperti Node.js atau PHP. Options:

#### Option 1: Manual Restart (Current)
```bash
# Setiap perubahan code:
go build -o bin/mylab-api-go ./cmd/mylab-api-go && sudo systemctl restart mylab-api-go
```

#### Option 2: Auto Rebuild dengan `air` (Development Only)
```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run dengan air (auto-rebuild on file change)
cd /var/www/mylab-api-go
air

# .air.toml config:
# root = "."
# tmp_dir = "tmp"
# [build]
#   cmd = "go build -o ./tmp/main ./cmd/mylab-api-go"
#   bin = "tmp/main"
#   include_ext = ["go"]
#   exclude_dir = ["tmp", "vendor"]
```

#### Option 3: Development Script
```bash
# scripts/dev.sh
#!/bin/bash
while true; do
    inotifywait -e modify,create,delete -r ./internal ./cmd && \
    go build -o bin/mylab-api-go ./cmd/mylab-api-go && \
    sudo systemctl restart mylab-api-go
done
```

## Logs

### View Logs
```bash
# Real-time logs
sudo journalctl -u mylab-api-go -f

# Last 100 lines
sudo journalctl -u mylab-api-go -n 100

# Logs from specific time
sudo journalctl -u mylab-api-go --since "10 minutes ago"

# Logs with error level
sudo journalctl -u mylab-api-go -p err
```

## Troubleshooting

### Service gagal start
```bash
# Cek error detail
sudo journalctl -u mylab-api-go -n 50 --no-pager

# Cek apakah binary valid
/var/www/mylab-api-go/bin/mylab-api-go --help

# Cek permission
ls -la /var/www/mylab-api-go/bin/mylab-api-go

# Test run manual
cd /var/www/mylab-api-go
HTTP_ADDR=:18080 DATABASE_URL=postgres://tiara:tiara@localhost:15432/mylab ./bin/mylab-api-go
```

### Port already in use
```bash
# Cek process yang pakai port 18080
sudo lsof -i :18080

# Kill process
sudo kill -9 <PID>

# Atau restart service
sudo systemctl restart mylab-api-go
```

### Database connection error
```bash
# Cek PostgreSQL running
docker ps | grep postgres

# Test connection
psql "postgres://tiara:tiara@localhost:15432/mylab" -c "SELECT 1"

# Start PostgreSQL container
cd /home/mylabapp/dockerdata
docker-compose up -d postgres
```

## Quick Reference

| Task | Command |
|------|---------|
| Build | `go build -o bin/mylab-api-go ./cmd/mylab-api-go` |
| Start | `sudo systemctl start mylab-api-go` |
| Stop | `sudo systemctl stop mylab-api-go` |
| Restart | `sudo systemctl restart mylab-api-go` |
| Status | `systemctl status mylab-api-go` |
| Logs | `sudo journalctl -u mylab-api-go -f` |
| Test | `curl http://localhost:18080/healthz` |
| Deploy | `go build -o bin/mylab-api-go ./cmd/mylab-api-go && sudo systemctl restart mylab-api-go` |
