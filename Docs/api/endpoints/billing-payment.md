# POST /v1/billing/payment

Save payment records for a billing transaction without modifying other billing data.

## Endpoint

```
POST /v1/billing/payment
```

## Description

This endpoint updates payment records for an existing billing transaction (`jual`). It:
- Updates payment records in `bdown_pay` table
- Recalculates header amounts in `jual` table
- Runs in a single transaction (all-or-nothing)

## Workflow Semantics

**Payment-only mode**: Updates payments and recalculates header amounts.

- Processes the payment records in the request
- Filters out invalid entries (e.g., zero amounts without ID)
- Updates `jual.bayar` and `jual.sisa` based on payment totals
- Does not modify `jual.total` or other billing line items

## Resource Payload

### Minimum Required Fields
- `no_lab` (string, required): Billing transaction number
- `payments` (array, required): Array of payment records (min 1)

### Optional Fields
- `id_karyawan` (string, optional): Employee/cashier ID
  - If omitted, uses `jual.kd_kasir` from the existing billing record
  - Must be provided if `jual.kd_kasir` is empty

### Payment Record Fields
Each payment record in the `payments` array supports:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string/number | No | Payment record ID (for updates) |
| `tanggal` | string | No | Payment date (YYYY-MM-DD) |
| `bayar` | number | No | Payment amount |
| `jnsbayar` | string | No | Payment method/type |
| `bank` | string | No | Bank name |
| `no_rek` | string | No | Account number |
| `nama_rek` | string | No | Account name |
| `rek_tujuan` | string | No | Destination account |

### Forced/Overridden Fields
The service **always sets** these fields automatically:
- `no_lab`: Forced from request root (not from payment record)
- `id_karyawan`: Forced from request root or `jual.kd_kasir`

### Defaults
- `tanggal`: Defaults to current date if omitted or empty
- `bayar`: Defaults to `0` if omitted or invalid
- Other fields: Empty string if omitted

### Filtering Rules
- Payment records with `bayar <= 0` and no `id` are **skipped** (prevents spam zero rows)
- Payment records with `id` are processed even if `bayar = 0` (allows updates/deletions)

## Request

### Headers
```
Content-Type: application/json
X-User-Id: 1
```

### Body Schema

```json
{
  "no_lab": "string",
  "id_karyawan": "string",
  "payments": [
    {
      "id": "string|number",
      "tanggal": "string",
      "bayar": number,
      "jnsbayar": "string",
      "bank": "string",
      "no_rek": "string",
      "nama_rek": "string",
      "rek_tujuan": "string"
    }
  ]
}
```

### Example Request

```json
{
  "no_lab": "LAB001",
  "id_karyawan": "KASIR01",
  "payments": [
    {
      "tanggal": "2026-01-16",
      "bayar": 500000,
      "jnsbayar": "TUNAI"
    },
    {
      "tanggal": "2026-01-16",
      "bayar": 300000,
      "jnsbayar": "TRANSFER",
      "bank": "BCA",
      "no_rek": "1234567890",
      "nama_rek": "PT Example"
    }
  ]
}
```

## Response

### Success (HTTP 200)

```json
{
  "ok": true,
  "message": "Pembayaran tersimpan.",
  "no_lab": "LAB001"
}
```

Note: the `message` value is returned by the service and may be localized.

### Validation Error (HTTP 422)

Missing required field:
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "no_lab": "no_lab is required."
  }
}
```

Empty payments array:
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "payments": "payments is required and must not be empty."
  }
}
```

Invalid payment amount:
```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "payments.0.bayar": "bayar must be numeric."
  }
}
```

### Not Found Error (HTTP 422)

```json
{
  "ok": false,
  "message": "Validation failed.",
  "errors": {
    "no_lab": "Billing record not found."
  }
}
```

### Conflict Error (HTTP 409)

Database unique constraint violation:
```json
{
  "ok": false,
  "message": "Conflict.",
  "errors": {
    "database": "Duplicate key violation."
  }
}
```

### Server Error (HTTP 500)

```json
{
  "ok": false,
  "message": "Internal server error."
}
```

## Validation Rules

### API-layer validation (fast fail)
- `no_lab`: Required, cannot be empty
- `payments`: Required array, must have at least 1 element
- `payments[].bayar`: Must be numeric (if provided)
- `id_karyawan`: Required if `jual.kd_kasir` is empty

### DB-layer validation (constraints)
- `no_lab`: Must exist in `jual` table (FK constraint)
- `id_karyawan`: Must be valid employee ID
- Transaction isolation: Prevents concurrent payment updates

## Examples

See complete request/response examples in:
- [examples/billing-payment-simple.json](../examples/billing-payment-simple.json)
- [examples/billing-payment-multiple.json](../examples/billing-payment-multiple.json)

## Related Endpoints

- (Coming soon) `POST /v1/billing/save` - Full billing save (desired-state mode)
- (Coming soon) `POST /v1/billing/append` - Append billing items (non-destructive)

## Notes

- This endpoint does **not** modify `jual.total` or billing line items
- Payment records are reconciled (old records not in request may be deleted)
- All operations run in a single transaction (atomicity guaranteed)
- Maximum request body size: 10MB
