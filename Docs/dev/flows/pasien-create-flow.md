# Flow: Pasien Create

**Endpoint**: `POST /v1/pasien`  
**Audience**: Developers working on pasien module

## Entry Point

Handler: `PasienHandlers.HandleCollection`  
Location: [internal/controllers/pasien/PasienController.go](../../../internal/controllers/pasien/PasienController.go#L1)

## Request Flow

1. Handler receives `POST /v1/pasien`.
2. Handler decodes JSON into a generic map (`map[string]any`).
   - Unknown fields are allowed by the handler.
3. Handler starts a DB transaction via `db.WithTx`.
4. Service call: `pasien.Service.Create(ctx, tx, payload)`.
5. Service delegates insert to `eloquent.Insert` using schema `pasienmodel.Schema()`.
6. Schema normalization:
   - Applies aliases (`company_id` → `sec_id`).
   - Filters to fillable fields (all columns except PK `kd_ps`).
   - Casts values according to schema casts.
7. Insert executes `INSERT ... RETURNING kd_ps`.
8. Transaction commits on success; rolls back on any error.
9. Handler returns HTTP 200 with `kd_ps`.

## Validation Layer

- JSON decode failure → HTTP 422 (`body: invalid JSON`).
- Payload cast failure in schema normalization → HTTP 422.
- Empty payload after filtering (no fillable fields) → HTTP 422.

## Database Operations

- Table: `pasien`
- Operation: `INSERT` with filtered/casted columns
- Primary key: `kd_ps` returned by DB
- Timestamps:
  - `created_at` and `updated_at` are set automatically if columns exist and not provided.

## Error Handling

- Validation errors (`eloquent.ValidationError`) → HTTP 422
- Not found errors are not applicable for create
- Other errors → HTTP 500

## Transaction Boundaries

- BEGIN: inside `db.WithTx`
- COMMIT: if insert returns successfully
- ROLLBACK: on any validation/DB error

## Related Code References

- Handler: [internal/controllers/pasien/PasienController.go](../../../internal/controllers/pasien/PasienController.go)
- Service: [internal/pasien/service.go](../../internal/pasien/service.go)
- Schema: [internal/database/model/pasienmodel/pasien_schema.go](../../internal/database/model/pasienmodel/pasien_schema.go)
- Insert helper: [internal/database/eloquent/crud.go](../../internal/database/eloquent/crud.go)
- Transaction wrapper: [internal/db/tx.go](../../internal/db/tx.go)
