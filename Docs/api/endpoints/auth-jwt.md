# Endpoint: JWT Auth Login

## POST /v1/auth/login

Authenticate user and return JWT token for API access.

### Request
- Content-Type: application/json
- Body:
```json
{
  "email": "user@example.com",
  "password": "yourpassword"
}
```

### Response (Success)
- Status: 200 OK
- Body:
```json
{
  "ok": true,
  "message": "Login successful.",
  "token": "<JWT token>",
  "expires_in": 86400,
  "expires_at": 1700000000
}
```

### Response (Error)
- Status: 401 Unauthorized / 422 Unprocessable Entity
- Body:
```json
{
  "ok": false,
  "message": "Unauthorized.",
  "errors": { "credentials": "invalid" }
}
```

---

# JWT Usage for API Requests

- After login, client must send JWT token in every request to protected endpoints:
  - Header: `Authorization: Bearer <JWT token>`

- Example:
```
GET /v1/crud/pasien/0000001
Authorization: Bearer <JWT token>
```

- If token is missing/invalid/expired, API returns 401 Unauthorized.

---

# JWT Token Structure
- Payload contains:
  - user_id
  - company_id
  - role
  - exp (expiry)

---

# Security Notes
- Token is signed with server secret key (should be set via ENV)
- Token expiry default: 24 hours
- Always use HTTPS in production

---

# Migration Notes
- X-User-Id header is no longer used for authentication.
- All protected endpoints now require JWT in Authorization header.
