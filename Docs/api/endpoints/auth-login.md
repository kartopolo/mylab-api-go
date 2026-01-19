# /v1/auth/login

User authentication endpoint for email/password login.

## Endpoint

- `POST /v1/auth/login` — Authenticate user with email and password

## Resource Payload

### Supported Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | User email address (case-insensitive) |
| `password` | string | Yes | User password (bcrypt-hashed in database) |

### Authentication

This endpoint is **public** and does not require authentication headers.

### Validation Rules

#### API-Layer Validation
- `email`: required, trimmed whitespace
- `password`: required, trimmed whitespace
- Unknown fields are rejected (strict JSON decoding)

#### Database-Layer Validation
- Email lookup is case-insensitive: `lower(email) = lower($1)`
- Password must match bcrypt hash stored in `users.password`
- Laravel bcrypt compatibility: `$2y$` prefix converted to `$2a$`

### Response

#### Success Response (200)
```json
{
  "ok": true,
  "message": "Login successful.",
  "token": "<JWT token>",
  "expires_in": 86400,
  "expires_at": 1768842974,
  "user_id": 123,
  "company_id": 45,
  "role": "admin"
}
```

**Note**: `role` field is only included if the user has a role set in the database.

#### Error Responses

**Validation Error (422)**
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "email": "required",
    "password": "required"
  }
}
```

**Invalid Credentials (401)**
```json
{
  "ok": false,
  "message": "Unauthorized.",
  "errors": {
    "credentials": "invalid"
  }
}
```

**Server Error (500)**
```json
{
  "ok": false,
  "message": "Internal server error."
}
```

## POST /v1/auth/login

Authenticate a user with email and password credentials.

### Request

Headers:
```
Content-Type: application/json
```

Body:
```json
{
  "email": "user@example.com",
  "password": "SecurePassword123"
}
```

### Response Examples

#### Success (200 OK)
```json
{
  "ok": true,
  "message": "Login successful.",
  "token": "<JWT token>",
  "expires_in": 86400,
  "expires_at": 1768842974,
  "user_id": 1,
  "company_id": 1,
  "role": "admin"
}
```

#### Missing Fields (422 Unprocessable Entity)
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "email": "required",
    "password": "required"
  }
}
```

#### Invalid Credentials (401 Unauthorized)
```json
{
  "ok": false,
  "message": "Unauthorized.",
  "errors": {
    "credentials": "invalid"
  }
}
```

#### Invalid JSON (422 Unprocessable Entity)
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "body": "invalid JSON"
  }
}
```

## Implementation Notes

### Password Hashing
- Database stores Laravel-compatible bcrypt hashes
- Hash prefix `$2y$` (Laravel/PHP) is automatically converted to `$2a$` (Go bcrypt)
- Password comparison uses `golang.org/x/crypto/bcrypt`

### Security Considerations
- Email lookup is case-insensitive for user convenience
- Failed login returns generic "invalid credentials" error (no information disclosure)
- Database errors return generic "Internal server error" (no information disclosure)
- No rate limiting implemented yet (consider adding for production)

### Database Schema
Expected `users` table columns:
- `id` (int64) - User ID
- `email` (string) - User email address
- `password` (string) - Bcrypt hash (nullable)
- `company_id` (int64) - Company/tenant ID
- `role` (string, nullable) - User role

## cURL Examples

### Successful Login
```bash
curl -X POST http://localhost:18080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "password123"
  }'
```

### Case-Insensitive Email
```bash
curl -X POST http://localhost:18080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "ADMIN@EXAMPLE.COM",
    "password": "password123"
  }'
```

### Missing Password
```bash
curl -X POST http://localhost:18080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com"
  }'
```

### Invalid Credentials
```bash
curl -X POST http://localhost:18080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "wrongpassword"
  }'
```

## Related Endpoints

After successful login, use the returned `user_id` in protected endpoints:

- All `/v1/*` endpoints (except `/v1/auth/login`) require authentication
- Authentication header: `X-User-Id: <user_id>`
- See [Authentication Middleware](../../dev/flows/auth-middleware-flow.md) for details

## See Also

- [JSON Examples](../examples/auth-login*.json)
- [OpenAPI Spec](../../openapi/openapi.yaml#/paths/~1v1~1auth~1login)
- [Auth Flow Documentation](../../dev/flows/auth-login-flow.md)
