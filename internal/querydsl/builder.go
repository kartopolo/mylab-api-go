package querydsl

import (
	"context"
	"fmt"
	"strings"

	"mylab-api-go/internal/database/eloquent"
)

type BuiltQuery struct {
	SQL  string
	Args []any
}

// BuildSQL validates QuerySpec using the provided Registry and builds a parameterized SQL query.
// It enforces tenant filtering by injecting `alias.company_id = companyID` for all tables that
// contain a `company_id` column.
func BuildSQL(ctx context.Context, reg *Registry, companyID int64, spec *QuerySpec) (*BuiltQuery, error) {
	_ = ctx
	if companyID <= 0 {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "invalid"}}
	}
	if spec == nil {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"query": "required"}}
	}
	if reg == nil {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"query": "registry missing"}}
	}

	baseSchema, ok := reg.Schema(spec.FromTable)
	if !ok {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"table": "unknown"}}
	}
	baseAlias := strings.TrimSpace(spec.FromAlias)
	if baseAlias == "" {
		baseAlias = spec.FromTable
	}

	aliasToTable := map[string]string{baseAlias: spec.FromTable}
	schemaByAlias := map[string]eloquent.Schema{baseAlias: baseSchema}

	// Register joins
	for i, j := range spec.Joins {
		if strings.TrimSpace(j.Table) == "" {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].table", i): "required"}}
		}
		alias := strings.TrimSpace(j.Alias)
		if alias == "" {
			alias = j.Table
		}
		if _, exists := aliasToTable[alias]; exists {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"join": "duplicate alias"}}
		}
		s, ok := reg.Schema(j.Table)
		if !ok {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].table", i): "unknown"}}
		}
		aliasToTable[alias] = j.Table
		schemaByAlias[alias] = s
	}

	// Helper to validate a ColumnRef against schemas
	validateCol := func(ref ColumnRef, fieldKey string) (ColumnRef, *eloquent.ValidationError) {
		alias := strings.TrimSpace(ref.Alias)
		col := strings.TrimSpace(ref.Column)
		if col == "" {
			return ColumnRef{}, &eloquent.ValidationError{Errors: map[string]string{fieldKey: "invalid column"}}
		}
		if alias == "" {
			alias = baseAlias
		}
		schema, ok := schemaByAlias[alias]
		if !ok {
			return ColumnRef{}, &eloquent.ValidationError{Errors: map[string]string{fieldKey: "unknown table alias"}}
		}
		if !schema.HasColumn(col) {
			return ColumnRef{}, &eloquent.ValidationError{Errors: map[string]string{fieldKey: "unknown field"}}
		}
		return ColumnRef{Alias: alias, Column: col}, nil
	}

	b := newSQLBuilder()

	// SELECT
	selectSQL := "*"
	if len(spec.Select) > 0 {
		cols := make([]string, 0, len(spec.Select))
		for i, raw := range spec.Select {
			ref, verr := validateCol(raw, fmt.Sprintf("select[%d]", i))
			if verr != nil {
				return nil, verr
			}
			cols = append(cols, ref.String())
		}
		selectSQL = strings.Join(cols, ",")
	}

	fromSQL := fmt.Sprintf("%s AS %s", spec.FromTable, baseAlias)

	// JOIN
	joinParts := make([]string, 0, len(spec.Joins))
	for i, j := range spec.Joins {
		alias := strings.TrimSpace(j.Alias)
		if alias == "" {
			alias = j.Table
		}
		left, verr := validateCol(j.On.Left, fmt.Sprintf("joins[%d].on.left", i))
		if verr != nil {
			return nil, verr
		}
		right, verr := validateCol(j.On.Right, fmt.Sprintf("joins[%d].on.right", i))
		if verr != nil {
			return nil, verr
		}
		if j.On.Op != "=" {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].on.op", i): "only '=' supported"}}
		}
		joinParts = append(joinParts, fmt.Sprintf("JOIN %s AS %s ON %s = %s", j.Table, alias, left.String(), right.String()))
	}

	// WHERE
	whereParts := make([]string, 0, 8)

	// Enforce tenant for all aliases that have company_id.
	for alias, schema := range schemaByAlias {
		if schema.HasColumn("company_id") {
			whereParts = append(whereParts, fmt.Sprintf("%s.company_id = %s", alias, b.push(companyID)))
		}
	}
	// If base table does not support tenant enforcement, reject.
	if !baseSchema.HasColumn("company_id") {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "schema does not support tenant filter (company_id missing)"}}
	}

	for i, w := range spec.Where {
		left, verr := validateCol(w.Left, fmt.Sprintf("where[%d].field", i))
		if verr != nil {
			return nil, verr
		}
		op := strings.ToLower(strings.TrimSpace(w.Op))
		switch op {
		case "=", "<=", ">=", "<", ">":
			whereParts = append(whereParts, fmt.Sprintf("%s %s %s", left.String(), op, b.push(w.Value)))
		case "like":
			whereParts = append(whereParts, fmt.Sprintf("%s ILIKE %s", left.String(), b.push(fmt.Sprintf("%%%v%%", w.Value))))
		default:
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("where[%d].op", i): "unsupported operator"}}
		}
	}

	// ORDER BY
	orderSQL := ""
	if len(spec.OrderBy) > 0 {
		parts := make([]string, 0, len(spec.OrderBy))
		for i, ob := range spec.OrderBy {
			field, verr := validateCol(ob.Field, fmt.Sprintf("order_by[%d].field", i))
			if verr != nil {
				return nil, verr
			}
			dir := strings.ToUpper(strings.ToLower(strings.TrimSpace(ob.Dir)))
			if dir == "" {
				dir = "ASC"
			}
			if dir != "ASC" && dir != "DESC" {
				return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("order_by[%d].dir", i): "must be asc or desc"}}
			}
			parts = append(parts, fmt.Sprintf("%s %s", field.String(), dir))
		}
		orderSQL = " ORDER BY " + strings.Join(parts, ",")
	}

	// LIMIT
	limitSQL := ""
	if spec.Limit > 0 {
		limitSQL = " LIMIT " + b.push(spec.Limit)
	}

	sql := fmt.Sprintf(
		"SELECT %s FROM %s %s WHERE %s%s%s",
		selectSQL,
		fromSQL,
		strings.Join(joinParts, " "),
		strings.Join(whereParts, " AND "),
		orderSQL,
		limitSQL,
	)

	return &BuiltQuery{SQL: sql, Args: b.args}, nil
}

// local sql builder to keep parameter numbering consistent
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
