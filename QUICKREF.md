# 🚀 MyLab API Quick Reference

## ✅ Status Service Saat Ini

```bash
Service: ✅ RUNNING (systemd)
Port:    18080
PID:     3723
Uptime:  ~1.5 hours
```

## 📝 Jawaban Pertanyaan

### 1. Apakah service jalan? 
**YA**, service sedang berjalan via systemd.

### 2. Setiap perubahan code harus rebuild?
**YA**, karena Go adalah compiled language, setiap perubahan code HARUS:
1. **Build** ulang binary
2. **Restart** service untuk load binary baru

### 3. Bagaimana client bisa akses?

#### Local (di server yang sama):
```bash
http://localhost:18080
```

#### Dari network lain:
```bash
# Cek IP server
ip addr show eth0 | grep "inet\b"

# Client akses via:
http://<SERVER_IP>:18080
```

#### Firewall:
Port 18080 sudah dibuka (TCP & UDP)

---

## ⚡ Quick Commands

### Deploy Setelah Edit Code
```bash
# Cara 1: Pakai Makefile (RECOMMENDED)
make deploy

# Cara 2: Pakai script
./scripts/deploy.sh

# Cara 3: Manual
go build -o bin/mylab-api-go ./cmd/mylab-api-go && \
sudo systemctl restart mylab-api-go
```

### Development dengan Auto-Reload
```bash
# Install inotify-tools dulu (sekali saja)
sudo apt install inotify-tools -y

# Jalankan watch mode
make dev
# atau
./scripts/dev-watch.sh
```

### Cek Status & Logs
```bash
# Status service
make status
# atau: systemctl status mylab-api-go

# Live logs
make logs
# atau: sudo journalctl -u mylab-api-go -f

# Error logs only
make logs-err

# Health check
make health
# atau: curl http://localhost:18080/healthz
```

---

## 🎯 Endpoint untuk Client

### Public Endpoints (no auth)
```bash
# Health check
GET http://localhost:18080/healthz

# Ready check
GET http://localhost:18080/readyz

# Metrics
GET http://localhost:18080/metrics
```

### Auth Endpoint
```bash
POST http://localhost:18080/v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

### Protected Endpoints (butuh X-User-Id header)
```bash
# Create Pasien
POST http://localhost:18080/v1/pasien
X-User-Id: 1
Content-Type: application/json

{
  "nm_ps": "John Doe",
  "alamat": "Jakarta",
  ...
}

# Get Pasien
GET http://localhost:18080/v1/pasien/{kd_ps}
X-User-Id: 1

# Update Pasien
PUT http://localhost:18080/v1/pasien/{kd_ps}
X-User-Id: 1
Content-Type: application/json

# Delete Pasien
DELETE http://localhost:18080/v1/pasien/{kd_ps}
X-User-Id: 1

# Select/Query Pasien
POST http://localhost:18080/v1/pasien/select
X-User-Id: 1
Content-Type: application/json

{
  "where": [["nm_ps", "like", "%John%"]],
  "page": 1,
  "per_page": 20
}

# Billing Payment
POST http://localhost:18080/v1/billing/payment
X-User-Id: 1
Content-Type: application/json
```

---

## 🏗️ Struktur Code (Post-Refactor)

```
internal/routes/
├── auth/           # Authentication module
│   ├── context.go
│   ├── handlers.go
│   └── middleware.go
├── billing/        # Billing module
│   └── handlers.go
├── pasien/         # Patient module
│   ├── handlers.go
│   └── select.go
├── shared/         # Shared utilities
│   ├── middleware.go
│   ├── request_id.go
│   └── response.go
└── server.go       # HTTP server setup
```

**Benefit**: Setiap modul terpisah, mudah maintain & trace!

---

## 📚 Dokumentasi Lengkap

- **Deployment Guide**: [DEPLOYMENT.md](DEPLOYMENT.md)
- **API Docs**: [Docs/api/](Docs/api/)
- **OpenAPI Spec**: [Docs/openapi/openapi.yaml](Docs/openapi/openapi.yaml)
- **Dev Flows**: [Docs/dev/flows/](Docs/dev/flows/)

---

## 🔧 Troubleshooting

### Service tidak bisa start
```bash
# Cek log error
sudo journalctl -u mylab-api-go -n 50

# Test manual run
cd /var/www/mylab-api-go
HTTP_ADDR=:18080 ./bin/mylab-api-go
```

### Port sudah dipakai
```bash
# Cek siapa yang pakai port 18080
sudo lsof -i :18080

# Kill process lama
sudo kill -9 <PID>

# Restart service
sudo systemctl restart mylab-api-go
```

### Database connection error
```bash
# Cek PostgreSQL running
docker ps | grep postgres

# Start PostgreSQL
cd /home/mylabapp/dockerdata
docker-compose up -d postgres
```

---

## 💡 Tips

1. **Selalu build + restart** setelah edit code
2. **Gunakan `make deploy`** untuk workflow cepat
3. **Pantau logs** saat testing: `make logs`
4. **Test health** setelah deploy: `make health`
5. **Development mode**: `make dev` untuk auto-reload

---

**Last Updated**: 2026-01-17
**Service Status**: ✅ Running on port 18080
