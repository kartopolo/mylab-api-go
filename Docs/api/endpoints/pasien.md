# /v1/pasien

CRUD endpoints for patient (`pasien`) records.

## Endpoints

- `POST /v1/pasien` — Create pasien
- `GET /v1/pasien/{kd_ps}` — Get pasien
- `PUT /v1/pasien/{kd_ps}` — Update pasien
- `DELETE /v1/pasien/{kd_ps}` — Delete pasien

## Resource Payload

### Supported Fields
This module uses a schema-driven payload filter:
- Supported fields are the fillable columns of the `pasien` table (all columns except the primary key `kd_ps`).
- Unknown fields are accepted in JSON but **ignored** (not persisted).

### Authentication

This endpoint requires a registered user.

- Header: `X-User-Id: <int>`

Tenant/company context is derived from `users.company_id`.

### Forced/Overridden Fields
- `kd_ps` is the primary key and is **not fillable**.
  - If provided in the request body, it is ignored.
  - The actual `kd_ps` is returned by the database on create.
- If the schema supports timestamps (`created_at`, `updated_at`):
  - `created_at` is set automatically on create when omitted.
  - `updated_at` is set automatically on create and is **forced** on update.

- `company_id` is forced to the authenticated user's `company_id`.
  - If provided by the client, it is ignored/overridden.

### Defaults
No field-level defaults are applied by the handler. Defaults come from:
- database defaults (if any)
- schema casting (invalid cast → validation error)

## POST /v1/pasien

Create a new pasien record.

### Request

Headers:
```
Content-Type: application/json
X-User-Id: 1
```

Body: JSON object (fillable fields only).

Example:
```json
{
  "nama_ps": "Budi Santoso",
  "jk": "L",
  "tgl_lahir": "1990-01-01T00:00:00Z",
  "alamat": "Jakarta",
  "no_hp": "08123456789",
  "email": "budi@example.com",
  "nik": "3173xxxxxxxxxxxx"
}
```

### Response (HTTP 200)

```json
{
  "ok": true,
  "message": "Pasien created.",
  "kd_ps": "PS0001"
}
```

### Errors

- Validation error (HTTP 422)
- Server error (HTTP 500)

## GET /v1/pasien/{kd_ps}

Get a pasien record by primary key.

### Response (HTTP 200)

```json
{
  "ok": true,
  "message": "OK",
  "data": {
    "kd_ps": "PS0001",
    "nama_ps": "Budi Santoso",
    "company_id": 1
  }
}
```

### Errors

- Not found (HTTP 404)
- Validation error (HTTP 422)
- Server error (HTTP 500)

## PUT /v1/pasien/{kd_ps}

Update a pasien record by primary key.

### Request

Headers:
```
Content-Type: application/json
X-User-Id: 1
```

Body: JSON object (fillable fields only).

Example:
```json
{
  "alamat": "Bandung",
  "no_hp": "08120000000"
}
```

### Response (HTTP 200)

```json
{
  "ok": true,
  "message": "Pasien updated.",
  "kd_ps": "PS0001"
}
```

### Errors

- Not found (HTTP 404)
- Validation error (HTTP 422)
- Server error (HTTP 500)

## DELETE /v1/pasien/{kd_ps}

Delete a pasien record by primary key.

### Response (HTTP 200)

```json
{
  "ok": true,
  "message": "Pasien deleted.",
  "kd_ps": "PS0001"
}
```

### Errors

- Not found (HTTP 404)
- Server error (HTTP 500)

## Unauthorized (HTTP 401)

Missing or invalid `X-User-Id`:
```json
{
  "ok": false,
  "message": "Unauthorized.",
  "errors": {
    "user_id": "missing X-User-Id header"
  }
}
```

## Examples

- [examples/pasien-create.json](../examples/pasien-create.json)
- [examples/pasien-get.json](../examples/pasien-get.json)
- [examples/pasien-update.json](../examples/pasien-update.json)
- [examples/pasien-delete.json](../examples/pasien-delete.json)
