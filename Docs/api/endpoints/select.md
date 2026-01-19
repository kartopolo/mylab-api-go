````markdown
# POST /v1/crud/{table}/select

Safe, schema-driven SELECT with filtering and pagination.

This endpoint is part of Generic CRUD. It loads a table schema (from `SCHEMA_DIR/{table}.txt` if present, otherwise via DB introspection), validates requested fields against known columns, enforces tenant isolation, and executes a parameterized query.

## Authentication

All `/v1/*` endpoints require `Authorization: Bearer <JWT>`.

Tenant context is derived from the authenticated user and enforced in SQL via a tenant column:

- Preferred: `company_id`
- Legacy fallback: `com_id`

If the table does not contain either column, the request is rejected.

## Request

### Path

- `table` (string): target table name (validated as `^[a-z0-9_]+$`).

### Body

The request body matches `eloquent.SelectRequest`.

```json
{
  "select": ["kd_ps", "nama_ps", "no_hp"],
  "where": {"jk": "L"},
  "or_where": {"sec_id": "10"},
  "like": {"nama_ps": "budi"},
  "or_like": {"alamat": "jakarta", "telepon": "0899%"},
  "order_by": [{"field": "created_at", "dir": "desc"}],
  "page": 1,
  "per_page": 25
}
```

### Fields

- `select` (array of string, optional)
  - If omitted or empty: selects **all columns** from the loaded schema.
  - Each entry must be a valid column (or an alias defined by schema `aliases=`).
  - Duplicates are removed.

- `where` (object, optional)
  - Equality filters: each key becomes `column = value`.
  - Keys must be valid columns (or schema aliases).

- `or_where` (object, optional)
  - Equality filters combined using `OR` inside a grouped expression.
  - Each key becomes `column = value`.
  - The group is AND-ed with other filters.

- `like` (object, optional)
  - Case-insensitive substring match (Postgres `ILIKE`).
  - Each key becomes `column ILIKE <pattern>`.
  - Pattern rules:
    - If client provides `%` or `_`, the server uses the pattern as-is.
    - Otherwise the server defaults to “contains” by wrapping as `%value%`.

- `or_like` (object, optional)
  - Like `like`, but combined using `OR` inside a grouped expression.
  - Use this for multi-column search (Laravel-style `orWhere` / `orWhereLike`).

- `order_by` (array, optional)
  - Each item:
    - `field` (string, required): column name (or schema alias)
    - `dir` (string, optional): `asc` or `desc` (default `asc`)

- `page` (int, optional)
  - Default: `1` when omitted or `<= 0`.

- `per_page` (int, optional)
  - Default: `100` when omitted or `<= 0`.
  - Max: `200`.

## Tenant Enforcement

The server always injects the tenant filter:

- If the schema contains `company_id`: `company_id = <companyID>`
- Else if the schema contains `com_id`: `com_id = <companyID>`

Clients do not need to (and should not) add tenant filtering in `where`.

## Query Shape (JSON → SQL)

The server builds a parameterized Postgres query using `$1..$N` placeholders.

Example request:

```json
{
  "select": ["kd_ps", "nama_ps", "alamat"],
  "where": {"jk": "L"},
  "or_like": {"nama_ps": "budi", "telepon": "0899%"},
  "page": 1,
  "per_page": 2
}
```

Conceptual SQL produced (simplified):

```sql
SELECT kd_ps,nama_ps,alamat
FROM pasien
WHERE company_id = $1
  AND jk = $2
  AND (nama_ps ILIKE $3 OR telepon ILIKE $4)
LIMIT $5 OFFSET $6
```

Args (in order):

```json
[
  10,
  "L",
  "%budi%",
  "0899%",
  3,
  0
]
```

Notes:

- `company_id`/`com_id` is always injected from the authenticated user (tenant enforcement).
- The server fetches `per_page + 1` rows to compute `has_more`.

## Validation Rules

- Unknown fields in JSON body are rejected (`DisallowUnknownFields`).
- Any unknown column in `select`, `where`, `like`, or `order_by[*].field` returns HTTP `422`.
- Invalid `order_by[*].dir` returns HTTP `422`.
- Tables not allowed by policy (`CRUD_DENIED_TABLES`) return HTTP `422`.

## Responses

### 200 OK

```json
{
  "ok": true,
  "message": "OK",
  "data": [
    {"kd_ps": "PS0001", "nama_ps": "Budi Santoso", "no_hp": "08123456789"}
  ],
  "paging": {
    "page": 1,
    "per_page": 25,
    "has_more": false,
    "total_rows": 1,
    "total_pages": 1
  }
}
```

### 401 Unauthorized

```json
{
  "ok": false,
  "message": "Unauthorized."
}
```

### 422 Validation failed

```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "nama_ps": "unknown field"
  }
}
```

### 500/503 Server error

```json
{
  "ok": false,
  "message": "Internal server error."
}
```

Notes:

- When the database connection is temporarily unavailable, the server may return HTTP `503` with message `Service unavailable.`.

## Examples

- Request example file: `Docs/api/examples/generic-crud-pasien-select.json`

### Multiple filters example

This example applies multiple `where` and multiple `like` filters. All conditions are combined using `AND`.

```json
{
  "select": ["kd_ps", "nama_ps", "jk", "no_hp", "sec_id"],
  "where": {
    "jk": "L",
    "sec_id": "10",
    "status": 1
  },
  "like": {
    "nama_ps": "budi",
    "alamat": "jakarta"
  },
  "order_by": [
    {"field": "nama_ps", "dir": "asc"},
    {"field": "kd_ps", "dir": "desc"}
  ],
  "page": 1,
  "per_page": 25
}
```

- Additional request example file: `Docs/api/examples/generic-crud-pasien-select-multiple.json`

- OR-LIKE multi-column search example file: `Docs/api/examples/generic-crud-pasien-select-or-like.json`

````
