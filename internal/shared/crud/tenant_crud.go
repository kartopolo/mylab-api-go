package crud

import (
	"context"
	"database/sql"

	"mylab-api-go/internal/database/eloquent"
)

// TenantCRUD menyediakan operasi CRUD standar yang tenant-aware (company_id).
// Ini sengaja dibuat berbasis schema + map payload, supaya konsisten dengan layer eloquent.
//
// PK adalah tipe primary key (contoh: string untuk pasien.kd_ps, int untuk menu.id).
type TenantCRUD[PK any] struct {
	schema func() eloquent.Schema
}

func NewTenantCRUD[PK any](schema func() eloquent.Schema) *TenantCRUD[PK] {
	return &TenantCRUD[PK]{schema: schema}
}

func (c *TenantCRUD[PK]) Create(ctx context.Context, tx *sql.Tx, companyID int64, payload map[string]any) (any, error) {
	schema := c.schema()
	// Force tenant from auth context.
	payload["company_id"] = companyID
	return eloquent.Insert(ctx, tx, schema, payload)
}

func (c *TenantCRUD[PK]) Get(ctx context.Context, tx *sql.Tx, companyID int64, pk PK) (map[string]any, error) {
	schema := c.schema()
	return eloquent.FindByPKAndCompanyID(ctx, tx, schema, pk, companyID)
}

func (c *TenantCRUD[PK]) Update(ctx context.Context, tx *sql.Tx, companyID int64, pk PK, payload map[string]any) error {
	schema := c.schema()
	// Force tenant from auth context.
	payload["company_id"] = companyID
	return eloquent.UpdateByPKAndCompanyID(ctx, tx, schema, pk, companyID, payload)
}

func (c *TenantCRUD[PK]) Delete(ctx context.Context, tx *sql.Tx, companyID int64, pk PK) error {
	schema := c.schema()
	return eloquent.DeleteByPKAndCompanyID(ctx, tx, schema, pk, companyID)
}

// List executes a paginated select using the schema bound to this TenantCRUD.
func (c *TenantCRUD[PK]) List(ctx context.Context, q eloquent.Querier, companyID int64, req eloquent.SelectRequest) (*eloquent.PageResult, error) {
	schema := c.schema()
	return eloquent.SelectPage(ctx, q, schema, companyID, req)
}
