# Feature: User Authentication (Login)

## Entry Point
Handler: `AuthController.HandleLogin` ([internal/controllers/auth/AuthController.go](../../../internal/controllers/auth/AuthController.go#L1))

## Request Flow

1. Handler receives `POST /v1/auth/login`
2. Validate: Method must be POST
3. Validate: Database connection is configured
4. Decode: JSON body to `loginRequest` struct (strict mode)
5. Normalize: Trim whitespace from email and password
6. Validate: Required fields (email, password)
7. Database lookup: Query user by email (case-insensitive)
8. Validate: User exists and has valid data
9. Validate: Password hash compatibility (Laravel bcrypt $2y$ → Go $2a$)
10. Validate: Password matches hash using bcrypt.CompareHashAndPassword
11. Response: Return user_id, company_id, and role (if exists)

## Validation Layer

### API-Layer Validation
- **Method**: Must be POST (405 if not)
- **Database**: Must be configured (500 if not)
- **JSON Decode**: Strict mode, unknown fields rejected (422 if invalid)
- **Required Fields**:
  - `email`: must not be empty after trim (422 if missing)
  - `password`: must not be empty after trim (422 if missing)

### Database-Layer Validation
- **User Lookup**: `SELECT id, company_id, role, password FROM users WHERE lower(email) = lower($1)`
  - Returns 401 if user not found
  - Returns 500 on database error
- **User Data**:
  - `user_id` and `company_id` must be > 0 (401 if not)
  - `password` hash must be valid and non-empty (401 if not)

### Password Validation
- **Hash Compatibility**: Convert Laravel `$2y$` prefix to Go-compatible `$2a$`
- **Hash Comparison**: bcrypt.CompareHashAndPassword
  - Returns 401 if password doesn't match

## Service Logic

No separate service layer - handler directly queries database for simplicity.

### Database Query
```sql
SELECT id, company_id, role, password 
FROM users 
WHERE lower(email) = lower($1) 
LIMIT 1
```

### Password Hash Normalization
```go
// Laravel bcrypt uses $2y$, Go bcrypt expects $2a$
if strings.HasPrefix(hash, "$2y$") {
    hash = "$2a$" + strings.TrimPrefix(hash, "$2y$")
}
```

### Password Verification
```go
err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
```

## Database Operations

### Tables Involved
- `users` (SELECT only)
  - Columns: `id`, `email`, `password`, `company_id`, `role`

### Transaction Handling
- **No transaction required** (read-only operation)
- Direct database query via `sqlDB.QueryRowContext`

## Error Handling

### Error Scenarios

| Scenario | HTTP Status | Response |
|----------|-------------|----------|
| Invalid method (not POST) | 405 | Method Not Allowed (empty body) |
| Database not configured | 500 | Internal server error |
| Invalid JSON | 422 | Validation failed (body: invalid JSON) |
| Missing email/password | 422 | Validation failed (field: required) |
| User not found | 401 | Unauthorized (credentials: invalid) |
| Invalid user_id/company_id | 401 | Unauthorized (credentials: invalid) |
| Missing password hash | 401 | Unauthorized (credentials: invalid) |
| Wrong password | 401 | Unauthorized (credentials: invalid) |
| Database query error | 500 | Internal server error |

### Security Considerations
- **No information disclosure**: All authentication failures return generic "invalid credentials"
- **Case-insensitive email**: User convenience without security impact
- **Timing attacks**: Not mitigated (consider constant-time comparison in production)
- **Rate limiting**: Not implemented (consider adding for production)

## Response Format

### Success (200)
```json
{
  "ok": true,
  "message": "Login successful.",
  "user_id": <int64>,
  "company_id": <int64>,
  "role": "<string>"  // Optional: omitted if null in database
}
```

### Validation Error (422)
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "<field>": "<reason>"
  }
}
```

### Unauthorized (401)
```json
{
  "ok": false,
  "message": "Unauthorized.",
  "errors": {
    "credentials": "invalid"
  }
}
```

### Server Error (500)
```json
{
  "ok": false,
  "message": "Internal server error."
}
```

## Related Code References
- [internal/controllers/auth/AuthController.go](../../../internal/controllers/auth/AuthController.go) (HandleLogin)
- [internal/routes/shared/response.go](../../../internal/routes/shared/response.go) (WriteError, WriteJSON)
- [internal/routes/server.go](../../../internal/routes/server.go) (Route registration)

## Usage in Client

After successful login, clients should:
1. Store `user_id` from response
2. Include `X-User-Id: <user_id>` header in all subsequent `/v1/*` requests (except `/v1/auth/login`)
3. Handle session management (not implemented by API - client responsibility)

Example:
```bash
# 1. Login
curl -X POST http://localhost:18080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"pass123"}'
# Response: {"ok":true,"user_id":1,"company_id":1,"role":"admin"}

# 2. Use authenticated endpoint
curl -X GET http://localhost:18080/v1/pasien/0000001 \
  -H "X-User-Id: 1"
```

## Future Improvements

1. **Token-based authentication**: Replace X-User-Id with JWT/session tokens
2. **Rate limiting**: Prevent brute-force attacks
3. **Account lockout**: Lock account after N failed attempts
4. **Audit logging**: Log all login attempts (success and failure)
5. **Multi-factor authentication**: Add 2FA support
6. **Password strength validation**: Enforce strong passwords
7. **Session management**: Implement server-side session tracking
8. **Refresh tokens**: Implement token refresh mechanism
