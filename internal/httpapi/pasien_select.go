package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"mylab-api-go/internal/database/eloquent"
	"mylab-api-go/internal/database/model/pasienmodel"
	"mylab-api-go/internal/db"
)

type PasienSelectHandlers struct {
	sqlDB *sql.DB
}

func NewPasienSelectHandlers(sqlDB *sql.DB) *PasienSelectHandlers {
	return &PasienSelectHandlers{sqlDB: sqlDB}
}

// POST /v1/pasien/select
// Body: eloquent.SelectRequest (safe SELECT only)
func (h *PasienSelectHandlers) HandleSelect(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/pasien/select" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.sqlDB == nil {
		writeError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
		return
	}

	var req eloquent.SelectRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	auth, ok := authInfoFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.", nil)
		return
	}

	schema := pasienmodel.Schema()
	res, err := db.WithTx(r.Context(), h.sqlDB, func(tx *sql.Tx) (*eloquent.PageResult, error) {
		return eloquent.SelectPage(r.Context(), tx, schema, auth.CompanyID, req)
	})
	if err != nil {
		var ve *eloquent.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, "Validation failed.", ve.Errors)
			return
		}
		writeError(w, http.StatusInternalServerError, "Internal server error.", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
