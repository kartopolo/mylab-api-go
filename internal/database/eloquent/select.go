package eloquent

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type OrderBy struct {
	Field string `json:"field"`
	Dir   string `json:"dir"` // asc|desc
}

type SelectRequest struct {
	Select    []string       `json:"select"`
	Where     map[string]any `json:"where"`
	Like      map[string]any `json:"like"`
	OrderBy   []OrderBy      `json:"order_by"`
	Page      int            `json:"page"`
	PerPage   int            `json:"per_page"`
}

type PageResult struct {
	Rows    []map[string]any
	Page    int
	PerPage int
	HasMore bool
}

const (
	DefaultPerPage = 25
	MaxPerPage     = 200
)

func SelectPage(ctx context.Context, q Querier, schema Schema, companyID int64, req SelectRequest) (*PageResult, error) {
	schema = schema.withDefaults()

	// Tenant enforcement (company_id)
	if companyID <= 0 {
		return nil, &ValidationError{Errors: map[string]string{"company_id": "invalid"}}
	}

	// Normalize select list
	selectCols, verr := normalizeSelect(schema, req.Select)
	if verr != nil {
		return nil, verr
	}

	page := req.Page
	perPage := req.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}
	offset := (page - 1) * perPage
	limit := perPage + 1 // fetch one extra to detect has_more

	builder := newSQLBuilder()
	whereParts := make([]string, 0, 8)

	// Always apply tenant filter as company_id
	if !schema.hasColumn("company_id") {
		return nil, &ValidationError{Errors: map[string]string{"company_id": "schema does not support tenant filter (company_id missing)"}}
	}
	whereParts = append(whereParts, builder.eq("company_id", companyID))

	// WHERE equals
	if req.Where != nil {
		keys := sortedKeys(req.Where)
		for _, k := range keys {
			col := resolveAlias(schema, k)
			if !schema.hasColumn(col) {
				return nil, &ValidationError{Errors: map[string]string{k: "unknown field"}}
			}
			if col == schema.PrimaryKey {
				// allow
			}
			whereParts = append(whereParts, builder.eq(col, req.Where[k]))
		}
	}

	// LIKE (case-insensitive on Postgres via ILIKE)
	if req.Like != nil {
		keys := sortedKeys(req.Like)
		for _, k := range keys {
			col := resolveAlias(schema, k)
			if !schema.hasColumn(col) {
				return nil, &ValidationError{Errors: map[string]string{k: "unknown field"}}
			}
			pattern := req.Like[k]
			whereParts = append(whereParts, builder.ilike(col, fmt.Sprintf("%%%v%%", pattern)))
		}
	}

	orderBySQL, verr := buildOrderBy(schema, req.OrderBy)
	if verr != nil {
		return nil, verr
	}

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s%s LIMIT %s OFFSET %s",
		strings.Join(selectCols, ","),
		schema.Table,
		strings.Join(whereParts, " AND "),
		orderBySQL,
		builder.arg(limit),
		builder.arg(offset),
	)

	rows, err := q.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0, perPage)
	for rows.Next() {
		m, err := scanCurrentRowToMap(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := false
	if len(out) > perPage {
		hasMore = true
		out = out[:perPage]
	}

	return &PageResult{Rows: out, Page: page, PerPage: perPage, HasMore: hasMore}, nil
}

func normalizeSelect(schema Schema, selectCols []string) ([]string, *ValidationError) {
	if len(selectCols) == 0 {
		// default: all columns
		return schema.Columns, nil
	}

	cols := make([]string, 0, len(selectCols))
	errs := map[string]string{}
	seen := map[string]bool{}
	for _, raw := range selectCols {
		col := resolveAlias(schema, raw)
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		if seen[col] {
			continue
		}
		seen[col] = true
		if !schema.hasColumn(col) {
			errs[raw] = "unknown field"
			continue
		}
		cols = append(cols, col)
	}
	if len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	if len(cols) == 0 {
		return nil, &ValidationError{Errors: map[string]string{"select": "empty"}}
	}
	return cols, nil
}

func buildOrderBy(schema Schema, orderBy []OrderBy) (string, *ValidationError) {
	if len(orderBy) == 0 {
		return "", nil
	}

	errs := map[string]string{}
	parts := make([]string, 0, len(orderBy))
	for i, ob := range orderBy {
		field := resolveAlias(schema, ob.Field)
		field = strings.TrimSpace(field)
		if field == "" {
			errs[fmt.Sprintf("order_by[%d].field", i)] = "required"
			continue
		}
		if !schema.hasColumn(field) {
			errs[fmt.Sprintf("order_by[%d].field", i)] = "unknown field"
			continue
		}
		dir := strings.ToLower(strings.TrimSpace(ob.Dir))
		if dir == "" {
			dir = "asc"
		}
		if dir != "asc" && dir != "desc" {
			errs[fmt.Sprintf("order_by[%d].dir", i)] = "must be asc or desc"
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", field, strings.ToUpper(dir)))
	}
	if len(errs) > 0 {
		return "", &ValidationError{Errors: errs}
	}
	return " ORDER BY " + strings.Join(parts, ","), nil
}

func resolveAlias(schema Schema, key string) string {
	k := strings.TrimSpace(key)
	if schema.Aliases != nil {
		if mapped, ok := schema.Aliases[k]; ok {
			return mapped
		}
	}
	return k
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type sqlBuilder struct {
	args []any
}

func newSQLBuilder() *sqlBuilder {
	return &sqlBuilder{args: make([]any, 0, 16)}
}

func (b *sqlBuilder) push(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}

func (b *sqlBuilder) arg(v any) string {
	return b.push(v)
}

func (b *sqlBuilder) eq(col string, v any) string {
	return fmt.Sprintf("%s = %s", col, b.push(v))
}

func (b *sqlBuilder) ilike(col string, v any) string {
	// Postgres-only operator; good enough for current docker env.
	return fmt.Sprintf("%s ILIKE %s", col, b.push(v))
}

func (b *sqlBuilder) secIDLegacyTenant(col string) string {
	// (col IS NULL OR col = '' OR col = '0')
	return fmt.Sprintf("(%s IS NULL OR %s = %s OR %s = %s)", col, col, b.push(""), col, b.push("0"))
}

