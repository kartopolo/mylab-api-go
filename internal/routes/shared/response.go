package shared

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	OK      bool              `json:"ok"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteOK(w http.ResponseWriter, message string) {
	WriteJSON(w, http.StatusOK, Envelope{OK: true, Message: message})
}

func WriteError(w http.ResponseWriter, status int, message string, errors map[string]string) {
	WriteJSON(w, status, Envelope{OK: false, Message: message, Errors: errors})
}
