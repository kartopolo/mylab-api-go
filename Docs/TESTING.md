# Testing MyLab API

This document covers various ways to test and debug the MyLab API.

## 1. VS Code REST Client (Recommended)

### Installation
1. Install extension: **REST Client** by Huachao Mao (`humao.rest-client`)
2. Or install: **Thunder Client** by Ranga Vadhineni (`rangav.vscode-thunder-client`)

### Usage with REST Client
1. Open file: [`api-tests.http`](../api-tests.http)
2. Click "Send Request" above any request
3. View response in split panel

**Features:**
- ✅ Syntax highlighting
- ✅ Variables support (`@baseUrl`, `@userId`)
- ✅ Environment switching
- ✅ Request history
- ✅ Response viewer with formatting

**Example:**
```http
### Login Test
POST http://localhost:18080/v1/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "password123"
}
```

Click "Send Request" link above the request!

## 2. Thunder Client (GUI Alternative)

Thunder Client provides a Postman-like GUI inside VS Code:

1. Install extension: `rangav.vscode-thunder-client`
2. Open Thunder Client panel (sidebar)
3. Create new request
4. Import from OpenAPI spec: `Docs/openapi/openapi.yaml`

**Features:**
- ✅ GUI interface
- ✅ Collections support
- ✅ Environment variables
- ✅ Request history
- ✅ OpenAPI import

## 3. Browser-Based Testing

### Simple GET Requests
Open browser and navigate directly:

```
http://localhost:18080/healthz
http://localhost:18080/metrics
http://localhost:18080/readyz
```

### Browser DevTools Console (POST Requests)

Open browser DevTools (F12) → Console tab:

```javascript
// Login request
fetch('http://localhost:18080/v1/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: 'admin@example.com',
    password: 'password123'
  })
})
.then(r => r.json())
.then(data => console.log(data));

// Get pasien (Generic CRUD, with JWT)
fetch('http://localhost:18080/v1/crud/pasien/0000001', {
  headers: { 'Authorization': 'Bearer <JWT token>' }
})
.then(r => r.json())
.then(data => console.log(data));

// Create pasien (Generic CRUD, with JWT)
fetch('http://localhost:18080/v1/crud/pasien', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer <JWT token>'
  },
  body: JSON.stringify({
    nm_ps: 'Test User',
    jk: 'L',
    alamat: 'Jakarta'
  })
})
.then(r => r.json())
.then(data => console.log(data));
```

### Browser Extensions

**Talend API Tester** (Chrome/Firefox):
1. Install from browser extension store
2. Import OpenAPI spec: `Docs/openapi/openapi.yaml`
3. Test endpoints with GUI

**Postman Web** (no install):
1. Visit https://web.postman.co
2. Import OpenAPI spec
3. Test endpoints

## 4. cURL (Command Line)

### Basic Requests
```bash
# Health check
curl http://localhost:18080/healthz

# Login
curl -X POST http://localhost:18080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'

# Get pasien (Generic CRUD)
curl http://localhost:18080/v1/crud/pasien/0000001 \
  -H "Authorization: Bearer <JWT token>"

# Create pasien (Generic CRUD)
curl -X POST http://localhost:18080/v1/crud/pasien \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT token>" \
  -d '{"nm_ps":"Test","jk":"L","alamat":"Jakarta"}'
```

### With jq (Pretty JSON)
```bash
curl -s http://localhost:18080/v1/auth/login \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}' \
  | jq '.'
```

## 5. Postman Desktop

1. Download from https://www.postman.com/downloads/
2. Import OpenAPI spec: `Docs/openapi/openapi.yaml`
3. Set environment variables:
   - `baseUrl`: `http://localhost:18080`
  - `token`: `<JWT token>`

## 6. Automated Testing with Go

Create test file: `internal/controllers/auth/AuthController_test.go` (or `internal/routes/auth/*_test.go` for middleware)

```go
package auth_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestLoginSuccess(t *testing.T) {
    // Setup test server
    // Send request
    // Verify response
}
```

Run tests:
```bash
go test ./internal/routes/auth/...
```

## 7. HTTP Files Collection

We provide ready-to-use `.http` files:

- [`api-tests.http`](../api-tests.http) - All endpoints
- Organized by category (auth, query, generic CRUD)
- Includes success and error cases

## Quick Test Workflow

### Initial Setup (One-Time)
```bash
# 1. Install REST Client extension in VS Code
# 2. Open api-tests.http
```

### Testing Flow
```bash
# 1. Ensure API is running
make health

# 2. Open api-tests.http in VS Code
# 3. Click "Send Request" above any test
# 4. View response in split panel
```

### Example Test Sequence
1. Test health: `GET /healthz`
2. Login: `POST /v1/auth/login`
3. Get pasien: `GET /v1/crud/pasien/0000001` (use token from login)
4. Create pasien: `POST /v1/crud/pasien`

## Troubleshooting

### CORS Errors in Browser
If testing from different origin:
```bash
# Set environment variable
export CORS_ALLOWED_ORIGINS="http://localhost:3000,http://localhost:8080"

# Restart service
make restart
```

### Connection Refused
```bash
# Check if service is running
make status

# Check port
netstat -tlnp | grep 18080

# Start service
make restart
```

### Unauthorized Errors
```bash
# Ensure Authorization header is present
# Check if user exists in database
psql "postgres://tiara:tiara@localhost:15432/mylab" -c "SELECT id FROM users LIMIT 5"
```

## Best Practices

1. **Use REST Client for development** - Fast and integrated with VS Code
2. **Use Thunder Client for complex workflows** - GUI is easier for sequences
3. **Use cURL for automation** - Scripts and CI/CD
4. **Use Postman for sharing** - Export collections for team
5. **Keep api-tests.http updated** - Add new tests when adding endpoints

## Documentation References

- [API Endpoints](api/README.md)
- [OpenAPI Spec](openapi/openapi.yaml)
- [JSON Examples](api/examples/)
- [Development Flows](dev/flows/)
