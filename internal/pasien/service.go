package pasien

import (
	"context"
	"database/sql"

	"mylab-api-go/internal/database/eloquent"
	"mylab-api-go/internal/database/model/pasienmodel"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Create(ctx context.Context, tx *sql.Tx, companyID int64, payload map[string]any) (any, error) {
	schema := pasienmodel.Schema()
	// Force tenant from auth context.
	payload["company_id"] = companyID
	return eloquent.Insert(ctx, tx, schema, payload)
}

func (s *Service) Get(ctx context.Context, tx *sql.Tx, companyID int64, kdPs string) (map[string]any, error) {
	schema := pasienmodel.Schema()
	return eloquent.FindByPKAndCompanyID(ctx, tx, schema, kdPs, companyID)
}

func (s *Service) Update(ctx context.Context, tx *sql.Tx, companyID int64, kdPs string, payload map[string]any) error {
	schema := pasienmodel.Schema()
	// Force tenant from auth context.
	payload["company_id"] = companyID
	return eloquent.UpdateByPKAndCompanyID(ctx, tx, schema, kdPs, companyID, payload)
}

func (s *Service) Delete(ctx context.Context, tx *sql.Tx, companyID int64, kdPs string) error {
	schema := pasienmodel.Schema()
	return eloquent.DeleteByPKAndCompanyID(ctx, tx, schema, kdPs, companyID)
}
