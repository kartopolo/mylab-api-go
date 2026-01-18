# /v1/crud/{table}

Generic, tenant-enforced CRUD endpoint.

This endpoint is intended to avoid duplicating controller code for standard CRUD.
The server loads schema from:

1) `SCHEMA_DIR/{table}.txt` (if present)
2) Database introspection (`information_schema`) as fallback

## Security

- Table access policy:
  - `CRUD_ALLOWED_TABLES` (allowlist, takes precedence, supports `*`)
  - `CRUD_DENIED_TABLES` (denylist, used only when allowlist is not set)
- Tenant enforcement: table must have `company_id` column.

## Endpoints

- `POST /v1/crud/{table}` — Create record
- `GET /v1/crud/{table}/{pk}` — Get record by PK
- `PUT /v1/crud/{table}/{pk}` — Update record
- `PATCH /v1/crud/{table}/{pk}` — Partial update (same as PUT, only provided fields)
- `DELETE /v1/crud/{table}/{pk}` — Delete record
- `POST /v1/crud/{table}/select` — List/select (safe filtering)

## Authentication

All `/v1/*` endpoints require `Authorization: Bearer <JWT>`.

## Request/Response

### Create

`POST /v1/crud/pasien`

Body: arbitrary JSON map (unknown keys are ignored by fillable rules; `company_id` is forced from JWT).

Response (200):
```json
{ "ok": true, "message": "Created.", "table": "pasien", "pk": "..." }
```

### Select

`POST /v1/crud/pasien/select`

Body: `eloquent.SelectRequest`
```json
{
  "where": {"jk": "L"},
  "like": {"nama_ps": "John"},
  "order_by": [{"field": "kd_ps", "dir": "desc"}],
  "page": 1,
  "per_page": 20
}
```

Response (200):
```json
{
  "ok": true,
  "message": "OK",
  "data": [{"kd_ps": "0001", "nama_ps": "John"}],
  "paging": {"page": 1, "per_page": 20, "has_more": false}
}
```

## Schema File Format (`SCHEMA_DIR/{table}.txt`)

Example: `pasien.txt`
```txt
# Minimal
primary_key=kd_ps

# Optional overrides
# timestamps=true
# aliases=com_id:company_id
# fillable=nama_ps,alamat,telepon
# columns=kd_ps,nama_ps,alamat,telepon,company_id,created_at,updated_at
# casts=company_id:int,created_at:datetime,updated_at:datetime
```
