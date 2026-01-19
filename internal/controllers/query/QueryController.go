package querycontroller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"mylab-api-go/internal/database/eloquent"
	"mylab-api-go/internal/db"
	"mylab-api-go/internal/querydsl"
	"mylab-api-go/internal/routes/auth"
	"mylab-api-go/internal/routes/shared"
)

type QueryController struct {
	sqlDB  *sql.DB
	policy querydsl.TablePolicy
}

type LaravelQueryRequest struct {
	LaravelQuery string `json:"laravel_query"`
}

// NewQueryController registers the allowed tables for safe query execution.
// Access is controlled purely by env policy; table/column existence is validated
// via information_schema (no hardcoded model list required).
func NewQueryController(sqlDB *sql.DB) *QueryController {
	// Configure table access via env vars (loaded from .env or system env).
	//
	// Denylist-only:
	// - QUERYDSL_DENIED_TABLES=menu
	// - If empty, all tables are allowed.
	// - Supports '*' meaning deny all tables.
	deniedRaw := strings.TrimSpace(os.Getenv("QUERYDSL_DENIED_TABLES"))

	policy := querydsl.ParseTablePolicy("", deniedRaw)
	return &QueryController{sqlDB: sqlDB, policy: policy}
}

// HandleQuery executes a safe, tenant-enforced query built from a restricted Laravel-style DSL.
// Endpoint: POST /v1/query
func (c *QueryController) HandleQuery(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/query" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if c.sqlDB == nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
		return
	}

	authInfo, ok := auth.AuthInfoFromContext(r.Context())
	if !ok {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", nil)
		return
	}

	var req LaravelQueryRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	spec, err := querydsl.ParseLaravelQuery(req.LaravelQuery)
	if err != nil {
		writeQueryError(w, err)
		return
	}
	// Default/cap limit to keep endpoint safe.
	if spec.Limit <= 0 {
		spec.Limit = 200
	}
	if spec.Limit > 200 {
		spec.Limit = 200
	}

	rows, err := db.WithTx(r.Context(), c.sqlDB, func(tx *sql.Tx) ([]map[string]any, error) {
		built, err := querydsl.BuildSQLWithIntrospection(r.Context(), tx, authInfo.CompanyID, spec, c.policy)
		if err != nil {
			return nil, err
		}
		rs, err := tx.QueryContext(r.Context(), built.SQL, built.Args...)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		return scanRowsToMaps(rs)
	})
	if err != nil {
		writeQueryError(w, err)
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "OK",
		"data":    rows,
	})
}

func writeQueryError(w http.ResponseWriter, err error) {
	var ve *eloquent.ValidationError
	if errors.As(err, &ve) {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", ve.Errors)
		return
	}
	shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", nil)
}

func scanRowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, 32)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			v := vals[i]
			// Convert []byte to string for JSON friendliness.
			if b, ok := v.([]byte); ok {
				m[c] = string(b)
				continue
			}
			m[c] = v
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
