package schema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mylab-api-go/internal/database/eloquent"
)

type columnQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// LoadSchema loads an eloquent.Schema for a table.
//
// Order:
// 1) File-based schema from SCHEMA_DIR/<table>.txt (if present)
// 2) DB introspection via information_schema (fallback)
//
// Notes:
// - This is designed to remove hardcoded Go model schema for standard CRUD.
// - Tenant enforcement requires company_id to exist in the table.
func LoadSchema(ctx context.Context, q columnQuerier, table string) (eloquent.Schema, error) {
	table = strings.ToLower(strings.TrimSpace(table))
	if table == "" {
		return eloquent.Schema{}, &eloquent.ValidationError{Errors: map[string]string{"table": "required"}}
	}
	if q == nil {
		return eloquent.Schema{}, &eloquent.ValidationError{Errors: map[string]string{"database": "not configured"}}
	}

	def, ok := tryLoadSchemaFile(table)
	if ok {
		// Fill missing parts by introspection if needed.
		return buildSchemaFromDefAndDB(ctx, q, table, def)
	}

	return buildSchemaFromDB(ctx, q, table)
}

type fileSchemaDef struct {
	PrimaryKey string
	Timestamps *bool
	Fillable   []string
	Columns    []string
	Aliases    map[string]string
	Casts      map[string]eloquent.CastType
}

func tryLoadSchemaFile(table string) (fileSchemaDef, bool) {
	dir := strings.TrimSpace(os.Getenv("SCHEMA_DIR"))
	if dir == "" {
		return fileSchemaDef{}, false
	}
	path := filepath.Join(dir, table+".txt")
	b, err := os.ReadFile(path)
	if err != nil {
		return fileSchemaDef{}, false
	}
	def, err := parseSchemaTXT(string(b))
	if err != nil {
		// treat parse error as "file not usable"; caller will fall back to DB
		return fileSchemaDef{}, false
	}
	return def, true
}

// parseSchemaTXT is a very small INI-like parser.
// Example:
// primary_key=kd_ps
// timestamps=true
// aliases=com_id:company_id
// fillable=nama_ps,alamat
// columns=kd_ps,nama_ps,alamat,company_id,created_at,updated_at
// casts=company_id:int,created_at:datetime
func parseSchemaTXT(raw string) (fileSchemaDef, error) {
	def := fileSchemaDef{Aliases: map[string]string{}, Casts: map[string]eloquent.CastType{}}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "primary_key", "pk":
			def.PrimaryKey = strings.TrimSpace(val)
		case "timestamps":
			v := strings.ToLower(strings.TrimSpace(val))
			b := v == "1" || v == "true" || v == "yes" || v == "y"
			def.Timestamps = &b
		case "fillable":
			def.Fillable = splitCSV(val)
		case "columns":
			def.Columns = splitCSV(val)
		case "aliases":
			// comma separated k:v
			for _, kv := range splitCSV(val) {
				p := strings.SplitN(kv, ":", 2)
				if len(p) != 2 {
					continue
				}
				k := strings.TrimSpace(p[0])
				v := strings.TrimSpace(p[1])
				if k == "" || v == "" {
					continue
				}
				def.Aliases[k] = v
			}
		case "casts":
			// comma separated col:type
			for _, kv := range splitCSV(val) {
				p := strings.SplitN(kv, ":", 2)
				if len(p) != 2 {
					continue
				}
				col := strings.TrimSpace(p[0])
				typ := strings.ToLower(strings.TrimSpace(p[1]))
				if col == "" || typ == "" {
					continue
				}
				switch typ {
				case string(eloquent.CastString):
					def.Casts[col] = eloquent.CastString
				case string(eloquent.CastInt):
					def.Casts[col] = eloquent.CastInt
				case string(eloquent.CastFloat):
					def.Casts[col] = eloquent.CastFloat
				case string(eloquent.CastBool):
					def.Casts[col] = eloquent.CastBool
				case string(eloquent.CastDateTime):
					def.Casts[col] = eloquent.CastDateTime
				default:
					// ignore unknown
				}
			}
		}
	}
	return def, nil
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func buildSchemaFromDefAndDB(ctx context.Context, q columnQuerier, table string, def fileSchemaDef) (eloquent.Schema, error) {
	schema, err := buildSchemaFromDB(ctx, q, table)
	if err != nil {
		return eloquent.Schema{}, err
	}

	if strings.TrimSpace(def.PrimaryKey) != "" {
		schema.PrimaryKey = strings.TrimSpace(def.PrimaryKey)
	}
	if len(def.Columns) > 0 {
		schema.Columns = def.Columns
	}
	if len(def.Fillable) > 0 {
		schema.Fillable = def.Fillable
	}
	if len(def.Aliases) > 0 {
		schema.Aliases = def.Aliases
	}
	if len(def.Casts) > 0 {
		schema.Casts = def.Casts
	}
	if def.Timestamps != nil {
		schema.Timestamps = *def.Timestamps
	}

	return schema, nil
}

func buildSchemaFromDB(ctx context.Context, q columnQuerier, table string) (eloquent.Schema, error) {
	cols, casts, err := introspectColumns(ctx, q, table)
	if err != nil {
		return eloquent.Schema{}, err
	}
	pk, err := introspectPrimaryKey(ctx, q, table)
	if err != nil {
		return eloquent.Schema{}, err
	}
	if pk == "" {
		return eloquent.Schema{}, &eloquent.ValidationError{Errors: map[string]string{"primary_key": "not found"}}
	}

	timestamps := false
	colSet := map[string]bool{}
	for _, c := range cols {
		colSet[strings.ToLower(c)] = true
	}
	if colSet["created_at"] && colSet["updated_at"] {
		timestamps = true
	}

	return eloquent.Schema{
		Table:      table,
		PrimaryKey: pk,
		Columns:    cols,
		Casts:      casts,
		Timestamps: timestamps,
		Now: func() time.Time {
			return time.Now()
		},
	}, nil
}

func introspectColumns(ctx context.Context, q columnQuerier, table string) ([]string, map[string]eloquent.CastType, error) {
	table = strings.ToLower(strings.TrimSpace(table))
	if table == "" {
		return nil, nil, &eloquent.ValidationError{Errors: map[string]string{"table": "required"}}
	}

	schemaName := strings.TrimSpace(os.Getenv("DB_SCHEMA"))
	if schemaName == "" {
		schemaName = "public"
	}

	rows, err := q.QueryContext(ctx,
		`SELECT column_name, data_type 
		 FROM information_schema.columns 
		 WHERE table_schema = $1 AND table_name = $2
		 ORDER BY ordinal_position`,
		schemaName, table,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := []string{}
	casts := map[string]eloquent.CastType{}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, nil, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		cols = append(cols, name)
		casts[name] = guessCastType(strings.TrimSpace(typ))
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(cols) == 0 {
		return nil, nil, &eloquent.ValidationError{Errors: map[string]string{"table": "not found"}}
	}
	return cols, casts, nil
}

func introspectPrimaryKey(ctx context.Context, q columnQuerier, table string) (string, error) {
	schemaName := strings.TrimSpace(os.Getenv("DB_SCHEMA"))
	if schemaName == "" {
		schemaName = "public"
	}

	rows, err := q.QueryContext(ctx,
		`SELECT kcu.column_name
		 FROM information_schema.table_constraints tc
		 JOIN information_schema.key_column_usage kcu
		   ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		 WHERE tc.constraint_type = 'PRIMARY KEY'
		   AND tc.table_schema = $1
		   AND tc.table_name = $2
		 ORDER BY kcu.ordinal_position`,
		schemaName, table,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var pk string
	if rows.Next() {
		if err := rows.Scan(&pk); err != nil {
			return "", err
		}
		pk = strings.TrimSpace(pk)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return pk, nil
}

func guessCastType(dbType string) eloquent.CastType {
	t := strings.ToLower(strings.TrimSpace(dbType))
	switch {
	case strings.Contains(t, "int"):
		return eloquent.CastInt
	case strings.Contains(t, "numeric"), strings.Contains(t, "decimal"), strings.Contains(t, "double"), strings.Contains(t, "real"), strings.Contains(t, "float"):
		return eloquent.CastFloat
	case strings.Contains(t, "bool"):
		return eloquent.CastBool
	case strings.Contains(t, "timestamp"), strings.Contains(t, "date"), strings.Contains(t, "time"):
		return eloquent.CastDateTime
	default:
		return eloquent.CastString
	}
}

var _ = errors.New
var _ = fmt.Sprintf
