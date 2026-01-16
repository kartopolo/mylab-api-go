package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"mylab-api-go/internal/observability"
)

type Server struct {
	httpServer *http.Server
}

func New(addr string, logLevelRaw string, sqlDB *sql.DB) *Server {
	mux := http.NewServeMux()
	metrics := observability.NewMetrics()
	level := parseLogLevel(logLevelRaw)

	billingHandlers := NewBillingHandlers(sqlDB)
	pasienHandlers := NewPasienHandlers(sqlDB)
	pasienSelectHandlers := NewPasienSelectHandlers(sqlDB)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeOK(w, "ok")
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
		if dbURL == "" {
			writeOK(w, "ready")
			return
		}

		if err := canDialDatabase(dbURL, 2*time.Second); err != nil {
			writeError(w, http.StatusServiceUnavailable, "Not ready.", map[string]string{"database": err.Error()})
			return
		}

		writeOK(w, "ready")
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.RenderPrometheus()))
	})

	mux.HandleFunc("/v1/billing/payment", billingHandlers.HandlePaymentOnly)
	mux.HandleFunc("/v1/pasien", pasienHandlers.HandleCollection)
	mux.HandleFunc("/v1/pasien/", pasienHandlers.HandleItem)
	mux.HandleFunc("/v1/pasien/select", pasienSelectHandlers.HandleSelect)

	srv := &http.Server{
		Addr:              addr,
		Handler:           withRecovery(withRequestID(withAuth(sqlDB, withAccessLog(level, withMetrics(metrics, mux))))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{httpServer: srv}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func canDialDatabase(rawURL string, timeout time.Duration) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	if parsed.Host == "" {
		return errors.New("DATABASE_URL host is empty")
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		switch strings.ToLower(parsed.Scheme) {
		case "postgres", "postgresql":
			port = "5432"
		case "mysql":
			port = "3306"
		default:
			return errors.New("DATABASE_URL port is empty")
		}
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
