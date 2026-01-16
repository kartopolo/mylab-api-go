//go:build legacy_disabled
// +build legacy_disabled

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"mylab-api-go/internal/billing"
	"mylab-api-go/internal/db"
)

type BillingHandlers struct {
	DB *db.DBDeps
}

// DBDeps is a small adapter so httpapi doesn't depend directly on *sql.DB details.
// It lives in internal/db to avoid circular deps.

func (h *BillingHandlers) HandlePaymentOnly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req billing.PaymentOnlyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	svc := billing.NewPaymentOnlyService()

	res, err := db.WithTx(r.Context(), h.DB.SQL, func(tx *db.Tx) (billing.PaymentOnlyResult, error) {
		return svc.SavePaymentOnly(r.Context(), tx.SQL, req)
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

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Payment saved.", "no_lab": res.NoLab})
}
