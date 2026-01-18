package serverdua

import (
	"net/http"
)

// RegisterRoutesDua digunakan untuk mendaftarkan endpoint tambahan (misal: fitur baru, prefix khusus, dsb)
func RegisterRoutesDua(mux *http.ServeMux) {
	// Tambahkan mux.HandleFunc di sini untuk endpoint baru
	// Contoh:
	// mux.HandleFunc("/v2/menu", menuHandlers.HandleCollection)
	// mux.HandleFunc("/v2/pasien", pasienHandlers.HandleCollection)
}
