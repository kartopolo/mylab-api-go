package eloquent

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func Insert(ctx context.Context, q Querier, schema Schema, payload map[string]any) (any, error) {
	schema = schema.withDefaults()
	data, verr := schema.normalizePayload(payload)
	if verr != nil {
		return nil, verr
	}

	if schema.Timestamps {
		now := schema.Now().UTC()
		if schema.hasColumn("created_at") {
			if _, ok := data["created_at"]; !ok {
				data["created_at"] = now
			}
		}
		if schema.hasColumn("updated_at") {
			if _, ok := data["updated_at"]; !ok {
				data["updated_at"] = now
			}
		}
	}

	cols, args := toSortedColsAndArgs(data)
	if len(cols) == 0 {
		return nil, &ValidationError{Errors: map[string]string{"payload": "no fillable fields provided"}}
	}

	placeholders := make([]string, 0, len(cols))
	for i := range cols {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		schema.Table,
		strings.Join(cols, ","),
		strings.Join(placeholders, ","),
		schema.PrimaryKey,
	)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("insert did not return primary key")
	}
	var pk any
	if err := rows.Scan(&pk); err != nil {
		return nil, err
	}
	return pk, nil
}

func FindByPK(ctx context.Context, q Querier, schema Schema, pk any) (map[string]any, error) {
	cols := schema.Columns
	if len(cols) == 0 {
		cols = []string{schema.PrimaryKey}
	}

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 LIMIT 1",
		strings.Join(cols, ","),
		schema.Table,
		schema.PrimaryKey,
	)

	rows, err := q.QueryContext(ctx, query, pk)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, &NotFoundError{Table: schema.Table, PK: pk}
	}

	m, err := scanCurrentRowToMap(rows)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func FindByPKAndCompanyID(ctx context.Context, q Querier, schema Schema, pk any, companyID int64) (map[string]any, error) {
	cols := schema.Columns
	if len(cols) == 0 {
		cols = []string{schema.PrimaryKey}
	}

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 AND company_id = $2 LIMIT 1",
		strings.Join(cols, ","),
		schema.Table,
		schema.PrimaryKey,
	)

	rows, err := q.QueryContext(ctx, query, pk, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		// Not found includes tenant mismatch; do not leak existence across tenants.
		return nil, &NotFoundError{Table: schema.Table, PK: pk}
	}

	m, err := scanCurrentRowToMap(rows)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// FindByPKAndTenant finds a record by primary key within a tenant boundary.
// tenantCol is typically "company_id" (preferred) or "com_id" (legacy).
func FindByPKAndTenant(ctx context.Context, q Querier, schema Schema, pk any, tenantCol string, tenantID int64) (map[string]any, error) {
	tenantCol = strings.TrimSpace(tenantCol)
	if tenantCol == "" {
		return nil, &ValidationError{Errors: map[string]string{"tenant": "tenant column required"}}
	}

	cols := schema.Columns
	if len(cols) == 0 {
		cols = []string{schema.PrimaryKey}
	}

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 AND %s = $2 LIMIT 1",
		strings.Join(cols, ","),
		schema.Table,
		schema.PrimaryKey,
		tenantCol,
	)

	rows, err := q.QueryContext(ctx, query, pk, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		// Not found includes tenant mismatch; do not leak existence across tenants.
		return nil, &NotFoundError{Table: schema.Table, PK: pk}
	}

	m, err := scanCurrentRowToMap(rows)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func UpdateByPK(ctx context.Context, q Querier, schema Schema, pk any, payload map[string]any) error {
	schema = schema.withDefaults()
	data, verr := schema.normalizePayload(payload)
	if verr != nil {
		return verr
	}

	if schema.Timestamps && schema.hasColumn("updated_at") {
		// Standard Eloquent behavior: updated_at is forced.
		data["updated_at"] = schema.Now().UTC()
	}

	cols, args := toSortedColsAndArgs(data)
	if len(cols) == 0 {
		return &ValidationError{Errors: map[string]string{"payload": "no fillable fields provided"}}
	}

	setParts := make([]string, 0, len(cols))
	for i, c := range cols {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", c, i+1))
	}
	args = append(args, pk)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d",
		schema.Table,
		strings.Join(setParts, ","),
		schema.PrimaryKey,
		len(args),
	)

	res, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return &NotFoundError{Table: schema.Table, PK: pk}
	}
	return nil
}

func UpdateByPKAndCompanyID(ctx context.Context, q Querier, schema Schema, pk any, companyID int64, payload map[string]any) error {
	schema = schema.withDefaults()
	data, verr := schema.normalizePayload(payload)
	if verr != nil {
		return verr
	}

	if schema.Timestamps && schema.hasColumn("updated_at") {
		// Standard Eloquent behavior: updated_at is forced.
		data["updated_at"] = schema.Now().UTC()
	}

	cols, args := toSortedColsAndArgs(data)
	if len(cols) == 0 {
		return &ValidationError{Errors: map[string]string{"payload": "no fillable fields provided"}}
	}

	setParts := make([]string, 0, len(cols))
	for i, c := range cols {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", c, i+1))
	}
	args = append(args, pk, companyID)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d AND company_id = $%d",
		schema.Table,
		strings.Join(setParts, ","),
		schema.PrimaryKey,
		len(args)-1,
		len(args),
	)

	res, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		// Not found includes tenant mismatch; do not leak existence across tenants.
		return &NotFoundError{Table: schema.Table, PK: pk}
	}
	return nil
}

// UpdateByPKAndTenant updates a record by primary key within a tenant boundary.
// tenantCol is typically "company_id" (preferred) or "com_id" (legacy).
func UpdateByPKAndTenant(ctx context.Context, q Querier, schema Schema, pk any, tenantCol string, tenantID int64, payload map[string]any) error {
	schema = schema.withDefaults()
	tenantCol = strings.TrimSpace(tenantCol)
	if tenantCol == "" {
		return &ValidationError{Errors: map[string]string{"tenant": "tenant column required"}}
	}

	data, verr := schema.normalizePayload(payload)
	if verr != nil {
		return verr
	}

	if schema.Timestamps && schema.hasColumn("updated_at") {
		// Standard Eloquent behavior: updated_at is forced.
		data["updated_at"] = schema.Now().UTC()
	}

	cols, args := toSortedColsAndArgs(data)
	if len(cols) == 0 {
		return &ValidationError{Errors: map[string]string{"payload": "no fillable fields provided"}}
	}

	setParts := make([]string, 0, len(cols))
	for i, c := range cols {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", c, i+1))
	}
	args = append(args, pk, tenantID)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d AND %s = $%d",
		schema.Table,
		strings.Join(setParts, ","),
		schema.PrimaryKey,
		len(args)-1,
		tenantCol,
		len(args),
	)

	res, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		// Not found includes tenant mismatch; do not leak existence across tenants.
		return &NotFoundError{Table: schema.Table, PK: pk}
	}
	return nil
}

func DeleteByPK(ctx context.Context, q Querier, schema Schema, pk any) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", schema.Table, schema.PrimaryKey)
	res, err := q.ExecContext(ctx, query, pk)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return &NotFoundError{Table: schema.Table, PK: pk}
	}
	return nil
}

func DeleteByPKAndCompanyID(ctx context.Context, q Querier, schema Schema, pk any, companyID int64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1 AND company_id = $2", schema.Table, schema.PrimaryKey)
	res, err := q.ExecContext(ctx, query, pk, companyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		// Not found includes tenant mismatch; do not leak existence across tenants.
		return &NotFoundError{Table: schema.Table, PK: pk}
	}
	return nil
}

// DeleteByPKAndTenant deletes a record by primary key within a tenant boundary.
// tenantCol is typically "company_id" (preferred) or "com_id" (legacy).
func DeleteByPKAndTenant(ctx context.Context, q Querier, schema Schema, pk any, tenantCol string, tenantID int64) error {
	tenantCol = strings.TrimSpace(tenantCol)
	if tenantCol == "" {
		return &ValidationError{Errors: map[string]string{"tenant": "tenant column required"}}
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1 AND %s = $2", schema.Table, schema.PrimaryKey, tenantCol)
	res, err := q.ExecContext(ctx, query, pk, tenantID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		// Not found includes tenant mismatch; do not leak existence across tenants.
		return &NotFoundError{Table: schema.Table, PK: pk}
	}
	return nil
}

func toSortedColsAndArgs(data map[string]any) ([]string, []any) {
	cols := make([]string, 0, len(data))
	for c := range data {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	args := make([]any, 0, len(cols))
	for _, c := range cols {
		args = append(args, data[c])
	}
	return cols, args
}

// Ensure sql package stays referenced (for lint sanity) and time imported is used.
var _ = sql.ErrNoRows
var _ = time.Second
