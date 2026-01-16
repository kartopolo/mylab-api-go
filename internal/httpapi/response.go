package httpapi

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	OK      bool              `json:"ok"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeOK(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusOK, Envelope{OK: true, Message: message})
}

func writeError(w http.ResponseWriter, status int, message string, errors map[string]string) {
	writeJSON(w, status, Envelope{OK: false, Message: message, Errors: errors})
}
