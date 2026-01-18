package querydsl

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"mylab-api-go/internal/database/eloquent"
)

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isSafeIdent(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && identRe.MatchString(s)
}

// BuildSQLWithIntrospection validates QuerySpec by checking real table/column names
// from information_schema, then builds a parameterized SQL query.
//
// Security:
// - Only safe identifiers are allowed for table/alias/column.
// - Only SELECT is generated.
// - Tenant filtering is enforced via injected `alias.company_id = companyID`.
func BuildSQLWithIntrospection(ctx context.Context, q columnQuerier, companyID int64, spec *QuerySpec, policy TablePolicy) (*BuiltQuery, error) {
	if companyID <= 0 {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "invalid"}}
	}
	if spec == nil {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"query": "required"}}
	}
	if q == nil {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"database": "not configured"}}
	}

	if !isSafeIdent(spec.FromTable) {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"table": "invalid"}}
	}
	if !policy.Allows(spec.FromTable) {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"table": "not allowed"}}
	}

	baseAlias := strings.TrimSpace(spec.FromAlias)
	if baseAlias == "" {
		baseAlias = spec.FromTable
	}
	if !isSafeIdent(baseAlias) {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"alias": "invalid"}}
	}

	aliasToTable := map[string]string{baseAlias: spec.FromTable}

	// Register joins first (and validate table/alias identifiers early).
	for i, j := range spec.Joins {
		table := strings.TrimSpace(j.Table)
		alias := strings.TrimSpace(j.Alias)
		if table == "" {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].table", i): "required"}}
		}
		if alias == "" {
			alias = table
		}
		if !isSafeIdent(table) {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].table", i): "invalid"}}
		}
		if !isSafeIdent(alias) {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].alias", i): "invalid"}}
		}
		if !policy.Allows(table) {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].table", i): "not allowed"}}
		}
		if _, exists := aliasToTable[alias]; exists {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"join": "duplicate alias"}}
		}
		aliasToTable[alias] = table
	}

	// Load columns for each referenced table.
	columnsByAlias := map[string]map[string]bool{}
	for alias, table := range aliasToTable {
		cols, err := loadTableColumns(ctx, q, table)
		if err != nil {
			return nil, err
		}
		columnsByAlias[alias] = cols
	}

	// Base table must support tenant enforcement.
	if !columnsByAlias[baseAlias]["company_id"] {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "table does not support tenant filter (company_id missing)"}}
	}

	validateCol := func(ref ColumnRef, fieldKey string) (ColumnRef, *eloquent.ValidationError) {
		alias := strings.TrimSpace(ref.Alias)
		col := strings.TrimSpace(ref.Column)
		if alias == "" {
			alias = baseAlias
		}
		if !isSafeIdent(alias) {
			return ColumnRef{}, &eloquent.ValidationError{Errors: map[string]string{fieldKey: "invalid table alias"}}
		}
		if !isSafeIdent(col) {
			return ColumnRef{}, &eloquent.ValidationError{Errors: map[string]string{fieldKey: "invalid column"}}
		}
		cols, ok := columnsByAlias[alias]
		if !ok {
			return ColumnRef{}, &eloquent.ValidationError{Errors: map[string]string{fieldKey: "unknown table alias"}}
		}
		if !cols[strings.ToLower(col)] {
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
		if strings.TrimSpace(j.On.Op) != "=" {
			return nil, &eloquent.ValidationError{Errors: map[string]string{fmt.Sprintf("joins[%d].on.op", i): "only '=' supported"}}
		}
		joinParts = append(joinParts, fmt.Sprintf("JOIN %s AS %s ON %s = %s", j.Table, alias, left.String(), right.String()))
	}

	// WHERE
	whereParts := make([]string, 0, 8)

	// Enforce tenant for all aliases that have company_id.
	for alias, cols := range columnsByAlias {
		if cols["company_id"] {
			whereParts = append(whereParts, fmt.Sprintf("%s.company_id = %s", alias, b.push(companyID)))
		}
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

	if len(whereParts) == 0 {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"where": "tenant filter missing"}}
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
