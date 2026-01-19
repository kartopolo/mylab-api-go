# Flow: Pasien Select (Paged)

**Endpoint**: `POST /v1/pasien/select`  
**Audience**: Developers working on pasien module

## Entry Point

Handler: `PasienSelectHandlers.HandleSelect`  
Location: [internal/controllers/pasien/PasienSelectController.go](../../../internal/controllers/pasien/PasienSelectController.go#L1)

## Purpose

This endpoint provides a safe, schema-validated paged `SELECT` for `pasien` records.

Key goals:
- Enforce tenant separation using `company_id` (mapped to `sec_id`)
- Prevent arbitrary SQL by validating fields against the schema
- Support common query needs (where/like/order_by/paging)

## Request Flow

1. Handler receives `POST /v1/pasien/select`.
2. Handler validates method and path.
3. Handler decodes JSON into `eloquent.SelectRequest`.
   - `UseNumber()` preserves numeric precision.
   - `DisallowUnknownFields()` rejects unknown top-level fields.
4. Handler loads schema: `pasienmodel.Schema()`.
5. Handler executes the select inside a DB transaction via `db.WithTx`.
6. Service logic runs in `eloquent.SelectPage(ctx, tx, schema, req)`:
   - Tenant enforcement (`company_id` required → always applies `sec_id` filter)
   - Field validation (select/where/like/order_by fields must exist in schema)
   - Paging normalization (defaults + max limits)
   - SQL building using parameter placeholders
7. Query executes and rows are scanned into `[]map[string]any`.
8. Handler returns HTTP 200 with `data` rows and `paging` metadata.

## Validation Layer

### Handler-level validation
- Invalid JSON → HTTP 422 with `body: invalid JSON`.
- Unknown top-level JSON fields → HTTP 422 (due to `DisallowUnknownFields`).

### Service-level validation (`eloquent.SelectPage`)
- `company_id` is required.
  - Missing/empty → HTTP 422 (`company_id: is required`).
- Schema must support tenant filtering.
  - If schema does not have `sec_id` column → HTTP 422.
- `select` field list must be valid.
  - Unknown field → HTTP 422 (`{field}: unknown field`).
  - Empty result after normalization → HTTP 422 (`select: empty`).
- `where` / `like` keys must exist in schema.
  - Unknown key → HTTP 422.
- `order_by` entries must be valid.
  - Unknown field → HTTP 422.
  - `dir` must be `asc` or `desc` → HTTP 422.

## Tenant Enforcement

Tenant filter is always applied as `sec_id`:

- If `company_id != "0"`:
  - SQL includes `sec_id = $X`.
- If `company_id == "0"`:
  - Uses a legacy-friendly filter that allows rows with empty/null `sec_id`.

This prevents cross-tenant data exposure.

## Database Operations

- Table: `pasien`
- Operation: `SELECT ... FROM pasien WHERE ... LIMIT ... OFFSET ...`
- Result scanning: each row becomes `map[string]any`

## Paging Behavior

- Default `page`: 1
- Default `per_page`: 100
- Max `per_page`: 200
- `has_more` is computed by fetching one extra row (`per_page + 1`) then trimming.
- `total_rows` and `total_pages` are computed using `COUNT(*)` with the same filters.

## Error Handling

- `eloquent.ValidationError` → HTTP 422
- Other errors (query/scan) → HTTP 500

## Transaction Boundaries

- BEGIN: inside `db.WithTx`
- COMMIT: after successful select
- ROLLBACK: on any error

Note: using a transaction for read ensures consistent behavior with the repo’s transaction wrapper pattern.

## Related Code References

- Handler: [internal/controllers/pasien/PasienSelectController.go](../../../internal/controllers/pasien/PasienSelectController.go)
- Request type: [internal/database/eloquent/select.go](../../internal/database/eloquent/select.go)
- Select logic: [internal/database/eloquent/select.go](../../internal/database/eloquent/select.go)
- Schema: [internal/database/model/pasienmodel/pasien_schema.go](../../internal/database/model/pasienmodel/pasien_schema.go)
- Transaction wrapper: [internal/db/tx.go](../../internal/db/tx.go)
