# MyLab API Documentation

This directory contains HTTP endpoint documentation and JSON payload examples for the MyLab API.

## Overview

MyLab API is a contract-first REST API built with Go for the MyLab medical laboratory system.

- **Base URL**: `http://localhost:18080` (development)
- **API Version**: `v1`
- **OpenAPI Contract**: [../openapi/openapi.yaml](../openapi/openapi.yaml)

## Standard API Envelope

All API responses follow a standard JSON envelope structure:

### Success Response (HTTP 200)
```json
{
  "ok": true,
  "message": "Success message.",
  "data": { }
}
```

### Validation Error (HTTP 422)
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "field_name": "Error reason."
  }
}
```

### Conflict Error (HTTP 409)
```json
{
  "ok": false,
  "message": "Conflict.",
  "errors": {
    "field_name": "Conflict reason."
  }
}
```

### Unauthorized (HTTP 401)
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

## Error Mapping Rules

| Error Type | HTTP Status | Description |
|-----------|-------------|-------------|
| API-layer validation failure | 422 | Required fields missing, invalid types, business rule violations |
| Database constraint violation (NOT NULL, CHECK, FK) | 422 | Database-level validation failure |
| Unique constraint violation | 409 | Duplicate key/unique index conflict |
| Schema mismatch | 500 | Unknown column/table (deployment/schema issue) |
| Generic database/server error | 500 | Unexpected server-side failure |

## Two-Layer Validation

The API implements a two-layer validation approach:

1. **API-layer validation (fast fail)**:
   - Validates required fields and basic types before touching the database
   - Returns HTTP 422 with field-level errors
   - Prevents unnecessary database operations

2. **DB-layer enforcement (safety net)**:
   - Database constraints (NOT NULL, UNIQUE, CHECK, FK) provide final validation
   - Catches edge cases that bypass API validation
   - Always triggers transaction rollback
   - Errors are mapped to stable API responses (422/409/500)

## Transactions

All multi-table workflows run inside a single database transaction:

```
BEGIN → writes → COMMIT (success)
                ↓
            ROLLBACK (on any error)
```

This ensures:
- **Atomicity**: All changes succeed or none do
- **Consistency**: Database state is always valid
- **Isolation**: Concurrent requests don't interfere
- **Durability**: Committed changes are permanent

## Available Endpoints

### Health & Observability (Public)
- `GET /healthz` - Basic health check
- `GET /readyz` - Readiness check (includes DB connectivity)
- `GET /metrics` - Prometheus metrics

### Authentication
- [`POST /v1/auth/login`](endpoints/auth-login.md) - User login (public endpoint)

### Protected Endpoints

All `/v1/*` endpoints (except `/v1/auth/login`) require authentication:

- **Header**: `X-User-Id: <int>`
- **Tenant context**: Derived from `users.company_id`
- **Unauthorized**: Returns HTTP 401 if header is missing or invalid

#### Billing
- [`POST /v1/billing/payment`](endpoints/billing-payment.md) - Save payment only

#### Generic CRUD (Table-based)
- [`POST /v1/crud/{table}`](endpoints/generic-crud.md) - Create record
- [`GET /v1/crud/{table}/{pk}`](endpoints/generic-crud.md) - Get record
- [`PUT /v1/crud/{table}/{pk}`](endpoints/generic-crud.md) - Update record
- [`PATCH /v1/crud/{table}/{pk}`](endpoints/generic-crud.md) - Partial update
- [`DELETE /v1/crud/{table}/{pk}`](endpoints/generic-crud.md) - Delete record
- [`POST /v1/crud/{table}/select`](endpoints/generic-crud.md) - Select/list (paged)

#### Query
- [`POST /v1/query`](endpoints/query.md) - Execute restricted query (Laravel-style DSL)

## JSON Examples

Complete request/response examples are available in the [examples/](examples/) directory.

## OpenAPI Contract

The complete API contract is defined in [../openapi/openapi.yaml](../openapi/openapi.yaml).
This file is the **source of truth** for all endpoints, request/response shapes, and validation rules.

## Development

When adding new endpoints:

1. Update OpenAPI contract first: `Docs/openapi/openapi.yaml`
2. Create endpoint documentation: `Docs/api/endpoints/{endpoint}.md`
3. Add JSON examples: `Docs/api/examples/{operation}.json`
4. Implement controller in `internal/controllers/` and register routes in `internal/routes/`
5. Ensure all three layers are consistent

## Support

For questions or issues, refer to the main [README.md](../../README.md) in the repository root.
