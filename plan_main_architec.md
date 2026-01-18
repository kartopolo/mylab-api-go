# Plan: Main Architecture (Go) — App Utama + Microservice Kecil Terpisah

Dokumen ini menjelaskan model arsitektur “app utama” (gateway) dengan service-service kecil (microservice/plugin service) yang terpisah, dengan gaya implementasi yang umum dan aman di Go.

> Target pembaca: developer internal.
> 
> Fokus: modularisasi fitur tanpa “rebuild semua” untuk perubahan tertentu, dengan tetap menjaga tenant boundary (`company_id`), keamanan, dan stabilitas API.

---

## 1) Tujuan

1. **Pisahkan fitur** sehingga sebagian logic bisa di-deploy/upgrade tanpa harus rebuild dan redeploy keseluruhan app utama.
2. **Kurangi risiko production**: perubahan besar cukup di microservice terkait.
3. **Tetap contract-first**: response envelope konsisten, error stabil.
4. **Aman untuk multi-tenant**: semua request terikat `company_id`.

---

## 2) Prinsip Standar Go

- **Compile-time binary**: Go menghasilkan binary statis; perubahan code → rebuild binary.
- Untuk “DLL-like behavior”, praktik yang paling aman adalah:
  - **Externalize config/metadata** (sudah dilakukan: `SCHEMA_DIR`, allow/deny table), dan/atau
  - **Pisahkan fitur sebagai service terpisah** (microservice) yang diakses lewat HTTP.
- Hindari `plugin` native (`.so`) untuk production kecuali benar-benar dikontrol ketat (Go version, build flags, ABI) karena rentan mismatch dan sulit operasi.

---

## 3) Gambaran Arsitektur

### 3.1 Komponen

1. **App Utama (Gateway)**: repository ini (`mylab-api-go`)
   - Menjadi “pintu depan” client.
   - Melakukan:
     - JWT verification
     - inject context (`user_id`, `company_id`, `role`)
     - rate limit (opsional)
     - request id, access log, metrics
     - routing ke handler internal atau ke microservice.

2. **Microservice (Plugin Service)**: service kecil terpisah per fitur
   - Contoh: `billing-worker`, `lab-reporting`, `inventory`, dll.
   - Menjalankan business logic yang tidak ingin ditaruh di app utama.
   - Bisa punya DB sendiri (recommended) atau share DB yang sama (allowed, tapi harus disiplin).

3. **Database**
   - Bisa shared database (satu DB dipakai app utama + microservice), atau per-service DB.
   - Jika shared DB: tetap wajib tenant filter (`company_id`) dan jangan lakukan query liar.

### 3.2 Alur Request

1. Client → **App Utama**
2. App Utama:
   - validasi JWT
   - tentukan tenant `company_id`
   - tentukan routing:
     - **internal handler** (yang sudah ada), atau
     - **forward** ke microservice sesuai konfigurasi
3. Microservice:
   - proses (DB/non-DB)
   - return JSON
4. App Utama → response ke client (pass-through atau normalisasi ringan)

---

## 4) Model Routing: “Plugin Registry” (Konsep DLL)

Agar mirip “drop DLL”, gunakan folder konfigurasi yang menentukan endpoint mana yang di-forward ke service mana.

### 4.1 Folder Config

- Env: `PLUGIN_DIR=/etc/mylab/plugins.d` (contoh)
- File: `*.json`
- App utama baca config pada startup (opsional: hot-reload dengan polling/FS notify).

### 4.2 Contoh Konfigurasi Plugin (JSON)

```json
{
  "name": "lab-reporting",
  "version": "1.0.0",
  "mount": "/v1/plugins/lab-reporting",
  "upstream": "http://127.0.0.1:19080",
  "timeout_ms": 30000,
  "auth_mode": "gateway_verified",
  "forward_headers": ["X-Request-Id"],
  "inject_headers": {
    "X-Plugin-Name": "lab-reporting"
  }
}
```

**Penjelasan**
- `mount`: prefix path yang dimiliki plugin.
- `upstream`: base URL microservice.
- `keep_mount_prefix` (optional, default `false`): jika `true`, gateway meneruskan path **tanpa** menghapus prefix `mount`.
- `auth_mode`:
  - `forward_jwt`: gateway forward `Authorization: Bearer ...` ke plugin; plugin verifikasi JWT sendiri.
  - `gateway_verified`: gateway verifikasi JWT, lalu kirim header internal (lihat bagian auth).

---

## 5) Keamanan & Auth (JWT) yang Aman

Ada 2 pola. Pilih salah satu sebagai standar.

### 5.1 Pola A — Forward JWT (paling sederhana, lebih independen)

- Gateway tetap verifikasi JWT untuk endpoint gateway internal.
- Untuk endpoint plugin:
  - gateway **forward** header `Authorization` apa adanya.
  - microservice **wajib** verifikasi JWT juga (share `JWT_SECRET` atau public key).

Kelebihan:
- Plugin bisa berdiri sendiri.

Kekurangan:
- Duplikasi proses verifikasi JWT.

### 5.2 Pola B — Gateway Verified + Internal Headers (lebih cepat, tapi harus network-trust)

- Gateway verifikasi JWT.
- Gateway inject header internal:
  - `X-User-Id`
  - `X-Company-Id`
  - `X-Role`
  - `X-Request-Id`
- Microservice **tidak menerima** header ini dari internet; hanya dari jaringan internal.

Wajib tambahan:
- Tambahkan **internal shared secret** (mis. `X-Internal-Signature`) atau mTLS antar service untuk mencegah spoofing.

---

## 6) Tenant Boundary (`company_id`) — Aturan Keras

- Setiap request yang menyentuh data tenant wajib punya `company_id`.
- Gateway wajib mengekstrak `company_id` dari JWT dan meneruskan ke plugin.
- Plugin wajib:
  - menggunakan `company_id` sebagai filter setiap query
  - menolak operasi kalau `company_id` kosong/invalid

---

## 7) Transaksi: Sinkron vs DB Transaction

### 7.1 Request Sinkron

Model request-response tetap sinkron:
- client → gateway → plugin → gateway → client

### 7.2 DB Transaction

- **DB transaction (BEGIN/COMMIT/ROLLBACK) hanya reliable di satu service**.
- Jangan mencoba “satu transaksi DB” melintasi gateway + plugin.

Jika ada workflow lintas service:
- Gunakan pola **Saga/outbox** (opsional) atau desain ulang supaya write dilakukan dalam satu service.

---

## 8) Kontrak API (Contract-First)

Agar gateway + plugin konsisten:

1. Setiap plugin harus punya OpenAPI sendiri (minimal), atau disatukan di gateway dengan “mounted paths”.
2. Response envelope konsisten:
   - Success: `{ "ok": true, "message": "...", ... }`
   - Error: `{ "ok": false, "message": "...", "errors": { ... } }`
3. Error mapping stabil:
   - 422 validation
   - 409 conflict
   - 500 internal

Rekomendasi:
- Gateway boleh pass-through response plugin.
- Gateway hanya menambahkan header korelasi: `X-Request-Id`.

---

## 9) Observability (Wajib Untuk Multi-Service)

### 9.1 Request ID

- Gateway generate `X-Request-Id` jika tidak ada.
- Forward `X-Request-Id` ke plugin.

### 9.2 Logging

- Semua service log JSON terstruktur.
- Minimal fields:
  - `request_id`, `path`, `method`, `status`, `latency_ms`, `company_id` (tanpa PII), `service`.

### 9.3 Metrics

- Expose `/metrics` per service (opsional), atau push ke central.
- Health check wajib: `GET /healthz`.

---

## 10) Deployment Model (Praktis)

### 10.1 Per-Service Binary + systemd

- Gateway: `mylab-api-go` (port 18080 → 8080 container)
- Plugin service: binary berbeda, port berbeda
- Contoh:
  - `lab-reporting` di `:19080`

### 10.2 Docker Compose

- Tambah service:
  - `mylab_api_go` (gateway)
  - `lab_reporting` (plugin)
- Pastikan 1 network yang sama.

---

## 11) Roadmap Implementasi (Bertahap)

### Tahap 1 — Minimal Forward Proxy di Gateway

- Tambah `PLUGIN_DIR` + parser config.
- Routing prefix `mount` → reverse proxy ke `upstream`.
- Forward headers:
  - `X-Request-Id`
  - `Authorization` (jika pilih pola A)
  - atau internal headers (jika pilih pola B)

### Tahap 2 — Standardisasi Kontrak Plugin

- Tetapkan standar endpoint:
  - `GET /healthz`
  - `GET /version`
- Tetapkan standar error envelope.

### Tahap 3 — Migrasi Fitur

- Pilih 1 fitur berisiko rendah untuk dipindah ke plugin terlebih dulu (mis. reporting).
- Gateway routing dipoint ke plugin.
- Setelah stabil, pindah fitur lain.

---

## 12) Batasan & Risiko

- Jika plugin share database yang sama:
  - risiko query tidak konsisten / race lebih tinggi
  - harus disiplin schema migration dan akses tabel
- Jika plugin butuh perubahan kontrak API:
  - harus sinkron dengan OpenAPI gateway
- Jika banyak plugin:
  - butuh governance (versi, monitoring, rolling restart)

---

## 13) Rekomendasi Default untuk MyLab

- Gunakan model: **Gateway (mylab-api-go) + Plugin Service (HTTP)**.
- Auth: mulai dari **Forward JWT (Pola A)** untuk sederhana.
- Tenant: `company_id` wajib di semua operasi data.
- Kontrak: response envelope konsisten; gateway pass-through.

---

## 14) Checklist Keputusan (Agar Implementasi Konsisten)

- [ ] `auth_mode` yang dipakai: `forward_jwt` atau `gateway_verified`
- [ ] Prefix plugin: `/v1/plugins/{name}` atau langsung `/v1/{feature}`
- [ ] Plugin boleh akses DB langsung atau hanya non-DB?
- [ ] Schema migrations dikelola di mana (gateway vs plugin)?
- [ ] Apakah butuh hot-reload untuk `PLUGIN_DIR`?

