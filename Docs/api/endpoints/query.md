# POST /v1/query

Execute a restricted, tenant-enforced query built from a safe subset of a Laravel-style query builder chain.

This endpoint does **not** execute raw SQL. The server parses the DSL, validates tables/columns via `information_schema`, injects tenant filters, and executes a parameterized query.

## Authentication

Required for all `/v1/*` endpoints.

## Request

### Body

```json
{
  "laravel_query": "table('menu as m')->select('m.id','m.menu_name')->where('m.menu_name','like','admin')->orderby('m.id','desc')->take(10)"
}
```

### Supported Methods (subset)

- `table('table as alias')`
- `select('a.col','a.col2',...)`
- `join('table as t','t.col','=','a.col')` (inner join only)
- `where('a.col','=','value')` or `where('a.col','value')`
- `where('a.col','<=','value')` (also `>=`, `<`, `>`, `like`)
- `orderby('a.col','asc|desc')`
- `take(1)` (limit, max 200)

### Restrictions

- Table access is controlled by denylist-only env policy:
  - `QUERYDSL_DENIED_TABLES`
  - If empty, all tables are allowed.
  - Use `*` to deny all tables.
- Any referenced table (and joined table) must have a `company_id` column (tenant enforcement).
- Unknown columns are rejected.
- Tenant filter is always enforced via `company_id`.
- Limit is capped to 200.

## Responses

### 200 OK

```json
{
  "ok": true,
  "message": "OK",
  "data": [
    {
      "id": 1,
      "menu_name": "Dashboard"
    }
  ]
}
```

### 422 Validation failed

```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "table": "unknown"
  }
}
```

### 401 Unauthorized

```json
{
  "ok": false,
  "message": "Unauthorized.",
  "errors": {
    "token": "missing or invalid Authorization header"
  }
}
```
