# POST /v1/pasien/select

Paged select endpoint for pasien records using a safe, schema-validated query object.

## Endpoint

```
POST /v1/pasien/select
```

## Description

This endpoint executes a paged `SELECT` against the `pasien` table with:
- Tenant enforcement via authenticated user's `users.company_id`
- Optional `where` filters (exact match)
- Optional `like` filters (case-insensitive match on Postgres)
- Optional ordering and column selection

## Resource Payload

### Authentication

This endpoint requires a registered user.

- Header: `X-User-Id: <int>`

The tenant/company context is derived from `users.company_id`.

### Optional Fields
- `select` (array of strings): Columns to return. If empty, returns all columns.
- `where` (object): Exact-match filters `{ field: value }`.
- `like` (object): Pattern-match filters `{ field: value }`.
- `order_by` (array): Ordering rules.
- `page` (int): Page number (default 1).
- `per_page` (int): Page size (default 25, max 200).

## Request

### Headers
```
Content-Type: application/json
X-User-Id: 1
```

### Body Schema

```json
{
  "select": ["field"],
  "where": {"field": "value"},
  "like": {"field": "value"},
  "order_by": [{"field": "field", "dir": "asc|desc"}],
  "page": 1,
  "per_page": 25
}
```

### Example Request

```json
{
  "select": ["kd_ps", "nama_ps", "no_hp", "sec_id"],
  "like": {
    "nama_ps": "budi"
  },
  "order_by": [
    {"field": "nama_ps", "dir": "asc"}
  ],
  "page": 1,
  "per_page": 25
}
```

## Response

### Success (HTTP 200)

```json
{
  "ok": true,
  "message": "OK",
  "data": [
    {"kd_ps": "PS0001", "nama_ps": "Budi Santoso", "no_hp": "08123456789", "sec_id": "10"}
  ],
  "paging": {
    "page": 1,
    "per_page": 25,
    "has_more": false
  }
}
```

### Validation Error (HTTP 422)
Unknown field in select/where/like/order_by:
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "some_field": "unknown field"
  }
}
```

### Unauthorized (HTTP 401)

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

### Server Error (HTTP 500)

```json
{
  "ok": false,
  "message": "Internal server error."
}
```

## Examples

- [examples/pasien-select.json](../examples/pasien-select.json)
