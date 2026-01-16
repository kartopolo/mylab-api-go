# MyLab API (Go) - AI Coding Instructions

## Purpose
- This repository is the HTTP API layer for MyLab.
- It exposes REST endpoints and talks to the database.
- Business workflows must be transactional and return stable, client-friendly errors.

## Scope & References (Behavior Accuracy)
- Goal: rebuild MyLab into a modern, scalable, contract-first REST API.
- Primary legacy behavior reference (read-only): `/var/www/mylab/app/Http/Controllers`.
  - Use this to confirm real-world workflow semantics and DB side effects.
- Secondary reference (preferred implementation/spec reference): `/var/www/mylab-core`.
  - Use this to port the cleaned-up business rules and payload/validation semantics.
- Always prefer `/var/www/mylab-core` for intended business rules and payload semantics.
- Use `/var/www/mylab/app/Http/Controllers` to confirm legacy production behavior (side effects, edge-cases) when uncertain.
- Do not use other workspace folders as references unless explicitly requested.

## Runtime / Compatibility
- Go 1.22+ (preferred).
- Must support MySQL and/or Postgres using Go `database/sql` drivers.
- The service must be stateless (no local session/state).

## Docker Environment & Resources
- **Location**: `/home/mylabapp/dockerdata/`
- **Compose files**: 
  - `docker-compose.yml` (base config)
  - `docker-compose.override.yml` (runtime overrides with mylab_api_go service)
- **Go Service**:
  - Service name: `mylab_api_go`
  - Build context: `/var/www/mylab-api-go` (this repo)
  - Image: Built from `Dockerfile` in this repo
  - Port: `18080` (maps to container `8080`)
  - Health check: `GET /healthz` endpoint
- **Database**:
  - Service: `postgres:15-alpine`
  - Host: `postgres` (container name, DNS within Docker network)
  - Port: `15432` (external), `5432` (internal)
  - Credentials: `POSTGRES_USER=tiara`, `POSTGRES_PASSWORD=tiara`
  - Database: `mylab`
  - Network: `custom_network` (bridge)
- **Environment Variables** (auto-set by docker-compose):
  - `HTTP_ADDR=${GO_HTTP_ADDR:-:8080}` (default `:8080` inside container)
  - `DATABASE_URL=postgres://tiara:tiara@postgres:5432/mylab?sslmode=disable`
  - `LOG_LEVEL=${GO_LOG_LEVEL:-info}`
- **Running locally**:
  - Build & start: `cd /home/mylabapp/dockerdata && docker-compose up --build mylab_api_go postgres`
  - Access API: `http://localhost:18080`
  - Access health: `http://localhost:18080/healthz`

## Architecture (Contract-first)
- OpenAPI is the source of truth: `Docs/openapi/openapi.yaml`.
- Endpoint handlers must follow the OpenAPI request/response shapes.
- Do not change payload field names in code without updating OpenAPI and JSON examples.
- All documentation must be **publish-ready** (valid for Postman/Swagger import).

### Contract vs Implementation (Conflict Resolution)
- If **code** and **OpenAPI** disagree, update **code** to match **OpenAPI**.
- If the **OpenAPI** is incorrect (confirmed by current implementation + intended behavior), update **OpenAPI**, **API docs**, and **JSON examples** in the same change.
- Never leave the repo in a state where only one of these is updated: handler/service, `Docs/openapi/openapi.yaml`, `Docs/api/`, `Docs/api/examples/`.

## Project Structure
- `cmd/mylab-api-go/main.go` — Service entry point (config, DB, HTTP server)
- `internal/config/` — Configuration loading (from env vars)
- `internal/db/` — Database connection & transaction management
- `internal/database/` — Eloquent-style query builder, scan helpers, error mapping
- `internal/httpapi/` — HTTP handlers, middleware, response formatting
- `Docs/openapi/openapi.yaml` — API contract (OpenAPI 3.1)
- `Docs/api/` — API documentation & JSON examples
- `Dockerfile` — Multi-stage build for production image

## Standard API Envelope
- Success: HTTP `200`
  - `{ "ok": true, "message": "...", ... }`
- Validation error: HTTP `422`
  - `{ "ok": false, "message": "Validation failed.", "errors": { "field": "reason" } }`
- Conflict error (e.g., unique key): HTTP `409`
  - `{ "ok": false, "message": "Conflict.", "errors": { ... } }`
- Server error: HTTP `500`
  - `{ "ok": false, "message": "Internal server error." }`

## Two-Layer Validation (Laravel-style, but Go)
1) API-layer validation (fast fail):
   - Validate required fields and basic types before touching DB.
   - Return HTTP `422` with field-level `errors`.
2) DB-layer enforcement (safety net):
   - Still catch database constraint errors (NOT NULL / FK / CHECK / UNIQUE).
   - Always rollback the transaction.
   - Map DB errors into stable API responses (422/409/500).

## Transactions (Workflows)
- All multi-table workflows MUST run inside a single DB transaction:
  - `BEGIN` → writes → `COMMIT`
  - On any error: `ROLLBACK` and return an error response.

### Transaction Boundaries (Concrete Rules)
- Decode/validate JSON **before** starting a transaction whenever possible.
- Inside a transaction, do only:
  - Required DB reads/writes for the workflow
  - Small deterministic calculations needed to persist correct values
- Never do inside a transaction:
  - Network calls (HTTP, external services)
  - Slow sleeps/retries/backoffs
  - Large I/O (file system) or long-running loops
  - Unbounded logging that may block

## Data Access
- Use `database/sql` with explicit SQL and small helper functions.
- Do not introduce a heavy ORM. Use the existing lightweight helpers under `internal/database/` when they reduce boilerplate without hiding SQL semantics.
- Keep SQL readable and parameterized; always use prepared statements/arguments.

## Error Mapping Rules
- Validation failures from API-layer checks → `422`.
- DB constraint violations:
  - NOT NULL / CHECK / FK violations → usually `422`.
  - UNIQUE / duplicate key → `409`.
  - Schema mismatch (unknown column/table) → `500` (deployment/schema issue).

## Observability
- Use structured JSON logs.
- Always include request id / correlation id in logs.
- On transaction failure, log enough context to debug safely (example: endpoint, no_lab, high-level error category).
- Never log secrets or sensitive payloads (credentials, tokens, full patient PII).

## When porting from mylab-core (PHP)
- The goal is identical outcomes (DB final state + response envelope), not identical code structure.
- Preserve the workflow semantics and validation rules from the reference implementation.
- Specific workflow behaviors (save/append/update modes) are documented in the API docs and OpenAPI contract.

## Documentation Rules (CRITICAL)
When creating or updating API documentation:

1. **Documentation as Valid Reference** (not rough examples):
   - All documentation must be **publish-ready** for Postman/Swagger/OpenAPI import
  - When asked to produce publish-ready docs, produce complete, valid documentation (do not leave TODO placeholders)
   - JSON examples must be valid and match current implementation

2. **Resource Payload Definition** (start with this):
   - Document supported fields (minimum required + optional)
   - Document fields that are **forced/overridden** by the service
   - Document fields that have **defaults** when omitted
   - Follow actual implementation from code (tables/services)

3. **Consistency Requirements**:
   - JSON examples must follow `*Table::RULES` (or equivalent validation in Go)
   - Field names must match actual database fields/payload accepted by handlers
   - If service does not compute values automatically, do not imply that it does
   - If code behavior changes, update related docs in the same commit

4. **Three-Layer Documentation** (always complete all three):
   - HTTP endpoint docs: `Docs/api/endpoints/*.md`
   - JSON examples: `Docs/api/examples/*.json`
   - OpenAPI contract: `Docs/openapi/openapi.yaml`
   - All three must be consistent and match the current implementation

5. **Language & Naming**:
   - All documentation must be **English-only**
   - Use actual field names from implementation (not legacy/UI names)
   - If strict validations exist, examples must pass validation

6. **Documentation Sync Triggers** (CRITICAL):
  - If you change a handler route, method, or response envelope → update OpenAPI + endpoint docs + examples.
  - If you change any request/response field name, type, or required/optional rule → update OpenAPI + endpoint docs + examples.
  - If you change validation rules or error mapping (422/409/500) → update endpoint docs + OpenAPI error schemas/examples.
  - If you change transactional behavior (tables touched, reconciliation rules, deletes) → update developer flow docs and endpoint docs.

## Development Documentation (Internal Flow)
When creating or updating development documentation for internal team understanding:

1. **Purpose & Audience**:
   - Audience: Developer working on the codebase
   - Purpose: Explain code flow, design decisions, and implementation details
   - Not for API consumers; internal use only

2. **Documentation Structure** (`Docs/dev/`):
   - `Docs/dev/architecture.md` — Overall system architecture and design patterns
   - `Docs/dev/flows/{feature-name}-flow.md` — Per-feature/endpoint execution flow
   - One flow document per feature/endpoint

3. **Flow Documentation Content** (mandatory sections):
   - **Feature Name & Purpose**: What does this feature do?
  - **Entry Point**: HTTP handler entry point for the endpoint (include file path and line number)
   - **Call Sequence**: Step-by-step method calls
     - Handler → Service → Database operations
     - Show the key functions/methods that define behavior (handler/service/DB), with file locations
     - Do not document every small helper; focus on the stable workflow steps and DB side effects
     - If an internal refactor changes helper names but behavior stays the same, update only the affected references
   - **Validation Layer**: API-layer validation steps
   - **Service Logic**: Business logic in service layer
   - **Database Operations**: SQL/transaction details
   - **Error Handling**: Error paths and response codes
   - **Transaction Boundaries**: BEGIN → COMMIT/ROLLBACK points
   - **Related Code References**: Links to actual files and line numbers

4. **Example Flow Template**:
   ```markdown
  # Feature: {Feature Name}
   
   ## Entry Point
  Handler: `{HandlerName}` ({path/to/file.go}:{line})
   
   ## Request Flow
  1. Handler receives `{METHOD} {PATH}`
  2. Validate: JSON decode + required fields + basic types
  3. Call: `{Service}.{Operation}(ctx, tx, req)`
  4. Service validates: business rules and normalization
  5. Service executes: required DB reads/writes and deterministic calculations
  6. Transaction: COMMIT (success) or ROLLBACK (error)
  7. Response: 200/422/409/500
   
   ## Transaction Handling
  - BEGIN transaction in: transaction wrapper
  - Writes: affected tables for this workflow
  - ROLLBACK on validation/DB error
  - COMMIT on success
   
   ## Error Scenarios
   - Validation error → 422 (API-layer or DB constraint)
  - Not found → 422 (required record not found)
   - Unique constraint → 409 (duplicate payment)
   - Server error → 500 (unexpected exception)
   ```

5. **Writing Standards**:
   - Use present tense ("Handler receives...", "Service validates...")
  - Include actual file paths and line numbers (update them when code moves)
   - Reference real code, not pseudocode
   - Explain "why" not just "what"
   - Keep flows concise and scannable

6. **Living Documentation** (CRITICAL):
   - Development documentation is a **living document** - it evolves with implementation
   - Every time you change code logic, architecture, or flow:
     - Update relevant flow document (`Docs/dev/flows/{feature}-flow.md`)
     - Update architecture document (`Docs/dev/architecture.md`) if patterns change
     - Update API documentation (`Docs/api/endpoints/{endpoint}.md`) if payload/behavior changes
   - Documentation must always reflect the **current state** of the code
   - Stale documentation is worse than no documentation - keep it in sync or it will mislead developers
   - Before committing code changes, verify:
     - Does flow documentation match the new code?
     - Did architecture patterns change? Update `architecture.md`
     - Did payload/validation change? Update API docs
   - If you're unsure what changed in the docs, **assume everything changed and review all three layers** (API docs + flow docs + architecture)

## Dependencies
- Do not add new dependencies unless requested.
- If a dependency is required (router/validator), keep it minimal and document why.
