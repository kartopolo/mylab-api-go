package crudcontroller

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"mylab-api-go/internal/database/eloquent"
	"mylab-api-go/internal/db"
	"mylab-api-go/internal/routes/auth"
	"mylab-api-go/internal/routes/shared"
	"mylab-api-go/internal/schema"
)

var tableNameRE = regexp.MustCompile("^[a-z0-9_]+$")

// TableCRUDController provides generic, tenant-enforced CRUD using table name.
//
// Routes:
// - POST   /v1/crud/{table}
// - GET    /v1/crud/{table}/{pk}
// - PUT    /v1/crud/{table}/{pk}
// - PATCH  /v1/crud/{table}/{pk}
// - DELETE /v1/crud/{table}/{pk}
// - POST   /v1/crud/{table}/select  (eloquent.SelectRequest)
//
// Security:
// - Table access is controlled by env policy: CRUD_ALLOWED_TABLES / CRUD_DENIED_TABLES.
// - Tenant enforcement uses company_id and rejects tables without company_id.
type TableCRUDController struct {
	sqlDB   *sql.DB
	denyAll bool
	denied  map[string]bool
}

func NewTableCRUDController(sqlDB *sql.DB) *TableCRUDController {
	deniedRaw := strings.TrimSpace(os.Getenv("CRUD_DENIED_TABLES"))

	c := &TableCRUDController{sqlDB: sqlDB, denied: map[string]bool{}}
	if deniedRaw != "" {
		for _, part := range strings.Split(deniedRaw, ",") {
			name := strings.ToLower(strings.TrimSpace(part))
			if name == "" {
				continue
			}
			if name == "*" {
				c.denyAll = true
				continue
			}
			c.denied[name] = true
		}
	}
	return c
}

func (c *TableCRUDController) Allows(table string) bool {
	t := strings.ToLower(strings.TrimSpace(table))
	if t == "" {
		return false
	}
	if c.denyAll {
		return false
	}
	return !c.denied[t]
}

func (c *TableCRUDController) Handle(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/crud/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if c.sqlDB == nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
		return
	}

	// Require auth to get tenant.
	authInfo, ok := auth.AuthInfoFromContext(r.Context())
	if !ok {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", nil)
		return
	}

	// /v1/crud/{table}[/...]
	path := strings.TrimPrefix(r.URL.Path, "/v1/crud/")
	path = strings.Trim(path, "/")
	segs := []string{}
	if path != "" {
		segs = strings.Split(path, "/")
	}
	if len(segs) == 0 {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"table": "required"})
		return
	}

	table := strings.ToLower(strings.TrimSpace(segs[0]))
	if table == "" {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"table": "required"})
		return
	}
	if !tableNameRE.MatchString(table) {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"table": "invalid name (allowed: a-z0-9_ only)"})
		return
	}

	if !c.Allows(table) {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"table": "not allowed"})
		return
	}

	// Optional subroute: /select
	if len(segs) == 2 && segs[1] == "select" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		c.handleSelect(w, r, authInfo.CompanyID, table)
		return
	}

	if len(segs) == 1 {
		// Collection: POST create only (safe default).
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		c.handleCreate(w, r, authInfo.CompanyID, table)
		return
	}

	// Item: {pk}
	pk := strings.TrimSpace(segs[1])
	if pk == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if strings.Contains(pk, "/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		c.handleGet(w, r, authInfo.CompanyID, table, pk)
		return
	case http.MethodPut, http.MethodPatch:
		c.handleUpdate(w, r, authInfo.CompanyID, table, pk)
		return
	case http.MethodDelete:
		c.handleDelete(w, r, authInfo.CompanyID, table, pk)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (c *TableCRUDController) handleCreate(w http.ResponseWriter, r *http.Request, companyID int64, table string) {
	var payload map[string]any
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	pk, err := db.WithTx(r.Context(), c.sqlDB, func(tx *sql.Tx) (any, error) {
		s, err := schema.LoadSchema(r.Context(), tx, table)
		if err != nil {
			return nil, err
		}
		tenantCol, verr := resolveTenantColumn(s)
		if verr != nil {
			return nil, verr
		}
		return eloquent.Insert(r.Context(), tx, s, withTenant(payload, tenantCol, companyID))
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Created.", "table": table, "pk": pk})
}

func (c *TableCRUDController) handleGet(w http.ResponseWriter, r *http.Request, companyID int64, table, pk string) {
	row, err := db.WithTx(r.Context(), c.sqlDB, func(tx *sql.Tx) (map[string]any, error) {
		s, err := schema.LoadSchema(r.Context(), tx, table)
		if err != nil {
			return nil, err
		}
		tenantCol, verr := resolveTenantColumn(s)
		if verr != nil {
			return nil, verr
		}
		return eloquent.FindByPKAndTenant(r.Context(), tx, s, pk, tenantCol, companyID)
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "OK", "data": row})
}

func (c *TableCRUDController) handleUpdate(w http.ResponseWriter, r *http.Request, companyID int64, table, pk string) {
	var payload map[string]any
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	_, err := db.WithTx(r.Context(), c.sqlDB, func(tx *sql.Tx) (any, error) {
		s, err := schema.LoadSchema(r.Context(), tx, table)
		if err != nil {
			return nil, err
		}
		tenantCol, verr := resolveTenantColumn(s)
		if verr != nil {
			return nil, verr
		}
		return nil, eloquent.UpdateByPKAndTenant(r.Context(), tx, s, pk, tenantCol, companyID, withTenant(payload, tenantCol, companyID))
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Updated.", "table": table, "pk": pk})
}

func (c *TableCRUDController) handleDelete(w http.ResponseWriter, r *http.Request, companyID int64, table, pk string) {
	_, err := db.WithTx(r.Context(), c.sqlDB, func(tx *sql.Tx) (any, error) {
		s, err := schema.LoadSchema(r.Context(), tx, table)
		if err != nil {
			return nil, err
		}
		tenantCol, verr := resolveTenantColumn(s)
		if verr != nil {
			return nil, verr
		}
		return nil, eloquent.DeleteByPKAndTenant(r.Context(), tx, s, pk, tenantCol, companyID)
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Deleted.", "table": table, "pk": pk})
}

func (c *TableCRUDController) handleSelect(w http.ResponseWriter, r *http.Request, companyID int64, table string) {
	var req eloquent.SelectRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	selectOnce := func() (*eloquent.PageResult, error) {
		return db.WithTx(r.Context(), c.sqlDB, func(tx *sql.Tx) (*eloquent.PageResult, error) {
		s, err := schema.LoadSchema(r.Context(), tx, table)
		if err != nil {
			return nil, err
		}
		if _, verr := resolveTenantColumn(s); verr != nil {
			return nil, verr
		}
		return eloquent.SelectPage(r.Context(), tx, s, companyID, req)
		})
	}

	res, err := selectOnce()
	if err != nil {
		// Retry once when the underlying tx connection is bad (common after DB restart).
		if errors.Is(err, driver.ErrBadConn) || strings.Contains(strings.ToLower(err.Error()), "driver: bad connection") {
			res, err = selectOnce()
		}
	}
	if err != nil {
		// Log detail for debugging (still return safe envelope to client).
		rid := shared.RequestIDFromContext(r.Context())
		log.Printf(
			`{"ts":%q,"level":"error","msg":"crud select failed","request_id":%q,"table":%q,"error":%q}`,
			time.Now().UTC().Format(time.RFC3339Nano),
			rid,
			table,
			err.Error(),
		)
		writeDomainError(w, r, err)
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "OK",
		"data":    res.Rows,
		"paging": map[string]any{
			"page":        res.Page,
			"per_page":    res.PerPage,
			"has_more":    res.HasMore,
			"total_rows":  res.TotalRows,
			"total_pages": res.TotalPages,
		},
	})
}

func withTenant(payload map[string]any, tenantCol string, companyID int64) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	payload[tenantCol] = companyID
	return payload
}

func resolveTenantColumn(s eloquent.Schema) (string, error) {
	if s.HasColumn("company_id") {
		return "company_id", nil
	}
	if s.HasColumn("com_id") {
		return "com_id", nil
	}
	return "", &eloquent.ValidationError{Errors: map[string]string{"tenant": "schema does not support tenant filter (company_id/com_id missing)"}}
}

func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	rid := ""
	if r != nil {
		rid = shared.RequestIDFromContext(r.Context())
	}

	var ve *eloquent.ValidationError
	if errors.As(err, &ve) {
		out := ve.Errors
		if out == nil {
			out = map[string]string{}
		}
		out["code"] = "validation_error"
		if rid != "" {
			out["request_id"] = rid
		}
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", out)
		return
	}

	var nf *eloquent.NotFoundError
	if errors.As(err, &nf) {
		errs := map[string]string{"id": "not found", "code": "not_found"}
		if rid != "" {
			errs["request_id"] = rid
		}
		shared.WriteError(w, http.StatusNotFound, "Not found.", errs)
		return
	}

	errCode := "internal_error"
	// Heuristic categorization (safe for UI; detail stays in logs).
	errLower := strings.ToLower(err.Error())
	status := http.StatusInternalServerError
	msg := "Internal server error."
	if strings.Contains(errLower, "driver: bad connection") {
		status = http.StatusServiceUnavailable
		msg = "Service unavailable."
		errCode = "db_bad_connection"
	} else if strings.Contains(errLower, "sqlstate") || strings.Contains(errLower, "failed to connect") || strings.Contains(errLower, "connection") {
		status = http.StatusServiceUnavailable
		msg = "Service unavailable."
		errCode = "database_error"
	}

	errs := map[string]string{"code": errCode}
	if rid != "" {
		errs["request_id"] = rid
	}
	shared.WriteError(w, status, msg, errs)
}
