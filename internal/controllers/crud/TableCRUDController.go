package crudcontroller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"

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
	sqlDB *sql.DB
	// allowlist precedence; supports "*".
	allowed       map[string]bool
	denied        map[string]bool
	allowAll      bool
	allowlistMode bool
}

func NewTableCRUDController(sqlDB *sql.DB) *TableCRUDController {
	allowedRaw := strings.TrimSpace(os.Getenv("CRUD_ALLOWED_TABLES"))
	deniedRaw := strings.TrimSpace(os.Getenv("CRUD_DENIED_TABLES"))

	c := &TableCRUDController{sqlDB: sqlDB, allowed: map[string]bool{}, denied: map[string]bool{}}
	if allowedRaw != "" {
		c.allowlistMode = true
		for _, part := range strings.Split(allowedRaw, ",") {
			name := strings.ToLower(strings.TrimSpace(part))
			if name == "" {
				continue
			}
			if name == "*" {
				c.allowAll = true
				continue
			}
			c.allowed[name] = true
		}
		return c
	}
	if deniedRaw != "" {
		for _, part := range strings.Split(deniedRaw, ",") {
			name := strings.ToLower(strings.TrimSpace(part))
			if name == "" {
				continue
			}
			c.denied[name] = true
		}
	}

	// Safety: if neither allowlist nor denylist configured, require explicit allowlist.
	// This is a safe-fail default to avoid accidental exposure.
	if allowedRaw == "" && deniedRaw == "" {
		c.allowlistMode = true
		// allowed remains empty -> no tables allowed until CRUD_ALLOWED_TABLES set.
	}
	return c
}

func (c *TableCRUDController) Allows(table string) bool {
	t := strings.ToLower(strings.TrimSpace(table))
	if t == "" {
		return false
	}
	if c.allowlistMode {
		if c.allowAll {
			return true
		}
		return c.allowed[t]
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
		if !s.HasColumn("company_id") {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "schema does not support tenant filter (company_id missing)"}}
		}
		return eloquent.Insert(r.Context(), tx, s, withTenant(payload, companyID))
	})
	if err != nil {
		writeDomainError(w, err)
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
		if !s.HasColumn("company_id") {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "schema does not support tenant filter (company_id missing)"}}
		}
		return eloquent.FindByPKAndCompanyID(r.Context(), tx, s, pk, companyID)
	})
	if err != nil {
		writeDomainError(w, err)
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
		if !s.HasColumn("company_id") {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "schema does not support tenant filter (company_id missing)"}}
		}
		return nil, eloquent.UpdateByPKAndCompanyID(r.Context(), tx, s, pk, companyID, withTenant(payload, companyID))
	})
	if err != nil {
		writeDomainError(w, err)
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
		if !s.HasColumn("company_id") {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "schema does not support tenant filter (company_id missing)"}}
		}
		return nil, eloquent.DeleteByPKAndCompanyID(r.Context(), tx, s, pk, companyID)
	})
	if err != nil {
		writeDomainError(w, err)
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

	res, err := db.WithTx(r.Context(), c.sqlDB, func(tx *sql.Tx) (*eloquent.PageResult, error) {
		s, err := schema.LoadSchema(r.Context(), tx, table)
		if err != nil {
			return nil, err
		}
		if !s.HasColumn("company_id") {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"company_id": "schema does not support tenant filter (company_id missing)"}}
		}
		return eloquent.SelectPage(r.Context(), tx, s, companyID, req)
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "OK",
		"data":    res.Rows,
		"paging": map[string]any{
			"page":     res.Page,
			"per_page": res.PerPage,
			"has_more": res.HasMore,
		},
	})
}

func withTenant(payload map[string]any, companyID int64) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["company_id"] = companyID
	return payload
}

func writeDomainError(w http.ResponseWriter, err error) {
	var ve *eloquent.ValidationError
	if errors.As(err, &ve) {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", ve.Errors)
		return
	}

	var nf *eloquent.NotFoundError
	if errors.As(err, &nf) {
		shared.WriteError(w, http.StatusNotFound, "Not found.", map[string]string{"id": "not found"})
		return
	}

	shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", nil)
}
