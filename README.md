# mylab-api-go

# mylab-api-go

Go-based REST API layer for MyLab.

- Contract-first: `Docs/openapi/openapi.yaml`
- Standard envelope: `ok/message/errors`
- Tenant enforced (JWT claim: `company_id`)

## Quick start

### 1) Configure env
```bash
cp .env.example .env
nano .env
```

Minimal required vars:
- `DATABASE_URL` (required for endpoints that hit DB)
- `JWT_SECRET` (required for auth)

### 2) Run

Option A (Docker compose - recommended for local dev):
```bash
cd /home/mylabapp/dockerdata
docker-compose up --build mylab_api_go postgres

curl http://localhost:58080/healthz
```

Option B (systemd / host run):
```bash
systemctl status mylab-api-go
curl http://localhost:18080/healthz
```

### 3) Auth (JWT)

Login:
```bash
curl -X POST http://localhost:18080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

All other `/v1/*` endpoints:
- `Authorization: Bearer <token>`

## Testing

- VS Code REST Client: [api-tests.http](api-tests.http)
- Guide: [Docs/TESTING.md](Docs/TESTING.md)

## Configuration

- Env reference: [Docs/CONFIGURATION.md](Docs/CONFIGURATION.md)
- Template: [.env.example](.env.example)

## Documentation

- OpenAPI: [Docs/openapi/openapi.yaml](Docs/openapi/openapi.yaml)
- Endpoint docs: [Docs/api/endpoints/](Docs/api/endpoints/)
- Examples: [Docs/api/examples/](Docs/api/examples/)
}).then(r => r.json()).then(console.log)
