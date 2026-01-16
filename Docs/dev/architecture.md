# MyLab API Architecture

Dokumen ini adalah **index flow diagram** (relasi antar proses) untuk dibaca cepat dan/atau dirender ke HTML.

Detail rule/validasi/payload per endpoint ada di `Docs/dev/flows/` dan `Docs/api/endpoints/`.

## Flow Index (Relasi)

```mermaid
flowchart TD
  Client[Client]

  Client --> BillingPayment[Billing: Payment]
  Client --> PasienCreate[Pasien: Create]
  Client --> PasienSelect[Pasien: Select (Paged)]
  Client --> PasienItem[Pasien: Get/Update/Delete]

  BillingPayment --> WithTx[db.WithTx (BEGIN/COMMIT/ROLLBACK)]
  PasienCreate --> WithTx
  PasienSelect --> WithTx
  PasienItem --> WithTx

  PasienCreate --> EloquentCRUD[eloquent: Insert/Update/Delete/Find]
  PasienItem --> EloquentCRUD
  PasienSelect --> EloquentSelect[eloquent: SelectPage]

  PasienSelect --> TenantFilter[Tenant filter: company_id/com_id → sec_id]

  WithTx --> DB[(Postgres)]
  EloquentCRUD --> DB
  EloquentSelect --> DB
  TenantFilter --> DB
```

## Flow Catalog

- Billing payment flow: [flows/billing-payment-flow.md](flows/billing-payment-flow.md)
- Pasien create flow: [flows/pasien-create-flow.md](flows/pasien-create-flow.md)
- Pasien select flow: [flows/pasien-select-flow.md](flows/pasien-select-flow.md)

## Catatan Tenant (Konsep)

- Tenant = company.
- User membawa `company_id`/`com_id`.
- Untuk tabel tenant-scoped, kolom database yang dipakai adalah `sec_id`.
- Kasus penting: data ada tapi `sec_id != company_id user` → proses harus gagal (umumnya diperlakukan seperti not found untuk tenant tsb).

## References

- API docs: [../api/README.md](../api/README.md)
- OpenAPI: [../openapi/openapi.yaml](../openapi/openapi.yaml)
- Flow docs: [flows/](flows/)
