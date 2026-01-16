package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mylab-api-go/internal/database/eloquent"
	"mylab-api-go/internal/db"
	"mylab-api-go/internal/pasien"
)

type PasienHandlers struct {
	sqlDB *sql.DB
}

func NewPasienHandlers(sqlDB *sql.DB) *PasienHandlers {
	return &PasienHandlers{sqlDB: sqlDB}
}

func (h *PasienHandlers) HandleCollection(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/pasien" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if h.sqlDB == nil {
		writeError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		var payload map[string]any
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&payload); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
			return
		}

		auth, ok := authInfoFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "Unauthorized.", nil)
			return
		}

		svc := pasien.NewService()
		pk, err := db.WithTx(r.Context(), h.sqlDB, func(tx *sql.Tx) (any, error) {
			return svc.Create(r.Context(), tx, auth.CompanyID, payload)
		})
		if err != nil {
			h.writeDomainError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Pasien created.", "kd_ps": pk})
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (h *PasienHandlers) HandleItem(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/pasien/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	kdPs := strings.TrimPrefix(r.URL.Path, "/v1/pasien/")
	kdPs = strings.TrimSpace(kdPs)
	if kdPs == "" || strings.Contains(kdPs, "/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if h.sqlDB == nil {
		writeError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
		return
	}

	svc := pasien.NewService()
	auth, ok := authInfoFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized.", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		row, err := db.WithTx(r.Context(), h.sqlDB, func(tx *sql.Tx) (map[string]any, error) {
			return svc.Get(r.Context(), tx, auth.CompanyID, kdPs)
		})
		if err != nil {
			h.writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "OK", "data": row})
		return
	case http.MethodPut:
		var payload map[string]any
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&payload); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
			return
		}

		_, err := db.WithTx(r.Context(), h.sqlDB, func(tx *sql.Tx) (any, error) {
			return nil, svc.Update(r.Context(), tx, auth.CompanyID, kdPs, payload)
		})
		if err != nil {
			h.writeDomainError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Pasien updated.", "kd_ps": kdPs})
		return
	case http.MethodDelete:
		_, err := db.WithTx(r.Context(), h.sqlDB, func(tx *sql.Tx) (any, error) {
			return nil, svc.Delete(r.Context(), tx, auth.CompanyID, kdPs)
		})
		if err != nil {
			h.writeDomainError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Pasien deleted.", "kd_ps": kdPs})
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (h *PasienHandlers) writeDomainError(w http.ResponseWriter, err error) {
	var ve *eloquent.ValidationError
	if errors.As(err, &ve) {
		writeError(w, http.StatusUnprocessableEntity, "Validation failed.", ve.Errors)
		return
	}

	var nf *eloquent.NotFoundError
	if errors.As(err, &nf) {
		writeError(w, http.StatusNotFound, "Not found.", map[string]string{"id": "not found"})
		return
	}

	writeError(w, http.StatusInternalServerError, "Internal server error.", nil)
}
