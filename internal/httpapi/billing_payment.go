package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"mylab-api-go/internal/billing"
	"mylab-api-go/internal/db"
)

type BillingHandlers struct {
	sqlDB *sql.DB
}

func NewBillingHandlers(sqlDB *sql.DB) *BillingHandlers {
	return &BillingHandlers{sqlDB: sqlDB}
}

func (h *BillingHandlers) HandlePaymentOnly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.sqlDB == nil {
		writeError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
		return
	}

	var req billing.PaymentOnlyRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	svc := billing.NewPaymentOnlyService()

	res, err := db.WithTx(r.Context(), h.sqlDB, func(tx *sql.Tx) (billing.PaymentOnlyResult, error) {
		return svc.SavePaymentOnly(r.Context(), tx, req)
	})
	if err != nil {
		var ve *billing.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, "Validation failed.", ve.Errors)
			return
		}

		writeError(w, http.StatusInternalServerError, "Internal server error.", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"message": "Pembayaran tersimpan.",
		"no_lab": res.NoLab,
	})
}
