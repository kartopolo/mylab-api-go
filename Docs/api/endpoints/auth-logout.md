# /v1/auth/logout

Logout endpoint for JWT-based sessions.

## Endpoint

- `POST /v1/auth/logout` — Revoke the current JWT token (best-effort)

## Authentication

This endpoint is **protected** and requires:

- Header: `Authorization: Bearer <JWT token>`

## Request

Headers:
```
Content-Type: application/json
Authorization: Bearer <JWT token>
```

Body (optional, can be `{}`):
```json
{}
```

## Response

### Success (200)
```json
{
  "ok": true,
  "message": "Logout successful."
}
```

### Unauthorized (401)
```json
{
  "ok": false,
  "message": "Unauthorized.",
  "errors": {
    "token": "missing or invalid Authorization header"
  }
}
```

## Notes

- Logout is implemented as token revocation (in-memory) until the JWT `exp`.
- Client/UI must still delete local token (clear session) to finish logout UX.

## cURL Example

```bash
curl -X POST http://localhost:18080/v1/auth/logout \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT token>" \
  -d '{}'
```
