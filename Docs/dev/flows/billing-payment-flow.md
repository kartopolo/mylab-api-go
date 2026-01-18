# Flow: Billing Payment Save

**Endpoint**: `POST /v1/billing/payment`  
**Feature**: Payment-only update (updates payments, recalculates header totals)  
**Audience**: Developers working on billing module

## Entry Point

**Handler**: `HandlePaymentOnly`  
**Location**: (removed) Billing module has been deleted.
**HTTP Method**: POST  
**Route**: `/v1/billing/payment`

## Request Flow

### 1. Handler Receives Request
**Location**: (removed)

```
POST /v1/billing/payment HTTP/1.1
Content-Type: application/json

{
  "no_lab": "LAB001",
  "id_karyawan": "KASIR01",
  "payments": [
    { "tanggal": "2026-01-16", "bayar": 500000, "jnsbayar": "TUNAI" }
  ]
}
```

**Handler Steps**:
- Check HTTP method (must be POST)
- Check database connection configured
- Decode JSON body into `PaymentOnlyRequest` struct
- Disallow unknown fields (security)

### 2. API-Layer Validation
**Location**: (removed)

Validates:
- JSON format (valid JSON, not malformed)
- Request body structure

**On Error**:
- Return HTTP 422 "Validation failed"
- Error: `"body": "invalid JSON"`

### 3. Create Service Instance
**Location**: (removed)

```go
svc := billing.NewPaymentOnlyService()
```

Service contains:
- Table names (`jual`, `bdown_pay`)
- Business logic methods

### 4. Execute in Transaction
**Location**: (removed)

```go
res, err := db.WithTx(r.Context(), h.sqlDB, func(tx *sql.Tx) (billing.PaymentOnlyResult, error) {
    return svc.SavePaymentOnly(r.Context(), tx, req)
})
```

**Transaction Wrapper** (`internal/db/tx.go`):
- Calls `BEGIN` on database
- Passes `*sql.Tx` to service callback
- **If callback succeeds**: `COMMIT`
- **If callback errors**: `ROLLBACK`

### 5. Service Validation & Processing
**Location**: `internal/billing/payment_only.go::SavePaymentOnly`

#### Step 5a: Normalize & Validate `no_lab`
```go
noLab := strings.ToUpper(strings.TrimSpace(req.NoLab))
if noLab == "" {
    return NewValidationError("no_lab is required.")
}
```

**Validation Rules**:
- Required (not empty)
- Trimmed and uppercase

**On Error**:
- Return `ValidationError` → Handler returns HTTP 422

#### Step 5b: Validate Payments Array
```go
if len(req.Payments) == 0 {
    return NewValidationError("payments is required and must not be empty.")
}
```

**Validation Rules**:
- Array must have at least 1 element

**On Error**:
- Return HTTP 422

#### Step 5c: Load Billing Header
**Location**: `internal/billing/payment_only.go::loadJualHeader` (line ~100)

```go
header, err := s.loadJualHeader(ctx, tx, noLab)
```

**SQL Query**:
```sql
SELECT no_lab, total, bayar, sisa, kd_kasir, ...
FROM jual
WHERE no_lab = ?
```

**Database Operation**:
- Runs in transaction (same `*sql.Tx`)
- **If not found**: Returns error
- Handler returns HTTP 422 "Billing record not found"

**Data Loaded**:
- `header.NoLab`: Billing number
- `header.KDKasir`: Employee ID (fallback)
- `header.Total`: Total amount
- `header.Bayar`: Current payment
- `header.Sisa`: Remaining balance

#### Step 5d: Determine Employee ID
```go
idKaryawan := strings.TrimSpace(req.IDKaryawan)
if idKaryawan == "" {
    idKaryawan = strings.TrimSpace(header.KDKasir)
}
if idKaryawan == "" {
    return NewValidationError("id_karyawan is required (or jual.kd_kasir must exist).")
}
```

**Logic**:
- Use request `id_karyawan` if provided
- Fall back to `jual.kd_kasir` if empty
- Fail if both empty

**On Error**: HTTP 422

#### Step 5e: Normalize & Filter Payment Records
**Location**: `internal/billing/payment_only.go::normalizePaymentRows` (line ~110)

For each payment in request:

```go
idStr := normalizeIDToString(p.ID)
bayarInt, bayarOK := normalizeInt(p.Bayar)
```

**Normalization**:
- `ID`: Convert to string (supports int/string)
- `Bayar`: Convert to integer, validate numeric

**Filtering Rules**:
- Skip if: `bayar <= 0` AND `id` is empty (prevents spam zero rows)
- Keep if: `id` exists (even if `bayar = 0`, allows updates/deletes)

**On Error**: Collect validation errors for field

#### Step 5f: Calculate Payment Totals
```go
// Calculate total bayar from filtered payments
totalBayar := 0
for _, p := range filtered {
    totalBayar += p.bayarInt
}
```

**Calculation**:
- Sum all valid payment amounts
- Store for header update

### 6. Database Operations

#### Step 6a: Update Jual Header
**Location**: `internal/billing/payment_only.go::updateJualHeader` (line ~150)

```go
err := s.updateJualHeader(ctx, tx, noLab, totalBayar, idKaryawan)
```

**SQL**:
```sql
UPDATE jual
SET bayar = ?, sisa = total - ?, kd_kasir = ?
WHERE no_lab = ?
```

**Parameters**:
- `bayar`: Total payment amount (calculated)
- `sisa`: `total - bayar` (remaining balance)
- `kd_kasir`: Employee ID
- `no_lab`: Billing number (WHERE clause)

**Database Constraints Applied**:
- NOT NULL checks
- FK constraints (if any)

#### Step 6b: Delete Old Payment Records
**Location**: `internal/billing/payment_only.go::deleteOldPayments` (line ~170)

```sql
DELETE FROM bdown_pay
WHERE no_lab = ?
AND id NOT IN (?, ?, ...)
```

**Logic**: 
- Keep only payment records in current request
- Delete records not in request (reconciliation)
- Prevents stale data

#### Step 6c: Insert/Update New Payment Records
**Location**: `internal/billing/payment_only.go::insertPaymentRows` (line ~180)

For each filtered payment:

```sql
INSERT INTO bdown_pay (no_lab, id, tanggal, bayar, jnsbayar, bank, no_rek, nama_rek, rek_tujuan, id_karyawan)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (no_lab, id) DO UPDATE SET ...
```

**OR** (UPDATE if exists):

```sql
UPDATE bdown_pay
SET tanggal = ?, bayar = ?, ...
WHERE no_lab = ? AND id = ?
```

**Fields Forced by Service**:
- `no_lab`: From request root (overwrites any value in request)
- `id_karyawan`: From request root or `jual.kd_kasir`

**Fields Defaulted**:
- `tanggal`: Current date if empty
- `bayar`: 0 if not numeric

### 7. Transaction Commit/Rollback
**Location**: `internal/db/tx.go::WithTx`

**If Service Succeeds** (no error):
```sql
COMMIT
```
- All changes persisted
- Billing record updated
- Payment records reconciled

**If Service Errors** (validation/DB error):
```sql
ROLLBACK
```
- All changes discarded
- Database state unchanged
- Error propagated to handler

### 8. Error Handling in Handler
**Location**: (removed)

**Error Categorization**:

```go
// Validation error
var ve *billing.ValidationError
if errors.As(err, &ve) {
    writeError(w, http.StatusUnprocessableEntity, "Validation failed.", ve.Errors)
    return
}

// Conflict error
var ce *billing.ConflictError
if errors.As(err, &ce) {
    writeError(w, http.StatusConflict, "Conflict.", ce.Errors)
    return
}

// Server error
writeError(w, http.StatusInternalServerError, "Internal server error.", nil)
```

**Error Response Codes**:
- `422`: Validation/not found errors
- `409`: Unique constraint conflicts
- `500`: Unexpected errors

### 9. Success Response
**Location**: (removed)

```go
writeOK(w, billing.PaymentOnlyResult{NoLab: res.NoLab})
```

**Response Structure**: use `internal/routes/shared/response.go`.

```json
{
  "ok": true,
  "message": "Payment saved successfully.",
  "no_lab": "LAB001"
}
```

HTTP Status: **200 OK**

## Transaction Boundaries

| Point | Operation | Transaction State |
|-------|-----------|-------------------|
| Step 4 | `db.WithTx()` calls | **BEGIN** |
| Step 5c | Load header | In transaction |
| Step 6a-c | Update/delete/insert | In transaction |
| Success | Return from callback | **COMMIT** |
| Error | Return error | **ROLLBACK** |

## Error Scenarios

### Scenario 1: JSON Parse Error
```
Client sends: { invalid json }
↓
Handler: JSON decode error
↓
Response: HTTP 422, "body": "invalid JSON"
↓
No transaction started (error before db.WithTx)
```

### Scenario 2: Required Field Missing
```
Client sends: { "payments": [] }
↓
Handler: Validation passes (JSON valid)
↓
Service: Checks payments array empty
↓
Response: HTTP 422, "payments": "payments is required and must not be empty."
↓
Transaction rolled back (if started)
```

### Scenario 3: Billing Record Not Found
```
Client sends: { "no_lab": "INVALID" }
↓
Service: loadJualHeader query returns no rows
↓
Response: HTTP 422, "no_lab": "Billing record not found."
↓
Transaction rolled back
```

### Scenario 4: Invalid Payment Amount
```
Client sends: { "bayar": "not a number" }
↓
Service: normalizeInt() fails
↓
Response: HTTP 422, "payments.0.bayar": "bayar must be numeric."
↓
Transaction rolled back
```

### Scenario 5: Unique Constraint Violation
```
Service: Inserts duplicate payment ID
↓
Database: UNIQUE constraint triggers
↓
Response: HTTP 409, "database": "Duplicate key violation."
↓
Transaction rolled back (auto by DB)
```

### Scenario 6: Success
```
All validations pass
↓
Database operations succeed
↓
Transaction: COMMIT
↓
Response: HTTP 200, "no_lab": "LAB001"
```

## Data Flow Summary

```
Request
  ↓
no_lab → [validate] → [load header] → jual record
  ↓
payments → [normalize] → [filter] → [calculate totals]
  ↓
id_karyawan → [validate/fallback] → use for all records
  ↓
Database Write (Transaction)
  ├─ UPDATE jual (bayar, sisa, kd_kasir)
  ├─ DELETE old payments
  └─ INSERT new payments
  ↓
Result: PaymentOnlyResult{NoLab: "LAB001"}
  ↓
Response: HTTP 200 + JSON
```

## Related Code References
- **Handler**: (removed) Billing module has been deleted.
- **Service**: (removed) Billing module has been deleted.
- **Transaction Wrapper**: [internal/db/tx.go](../../internal/db/tx.go)
- **Response Helpers**: [internal/routes/shared/response.go](../../internal/routes/shared/response.go)
- **Error Types**: (removed) Billing module has been deleted.

## API Documentation

See also:
- **API Docs**: [../../Docs/api/endpoints/billing-payment.md](../../Docs/api/endpoints/billing-payment.md)
- **OpenAPI**: [../../Docs/openapi/openapi.yaml](../../Docs/openapi/openapi.yaml)
- **JSON Examples**: [../../Docs/api/examples/](../../Docs/api/examples/)
