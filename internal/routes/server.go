package routes

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

	authcontroller "mylab-api-go/internal/controllers/auth"
	crudcontroller "mylab-api-go/internal/controllers/crud"
	pluginscontroller "mylab-api-go/internal/controllers/plugins"
	querycontroller "mylab-api-go/internal/controllers/query"
	"mylab-api-go/internal/observability"
	"mylab-api-go/internal/routes/auth"
	"mylab-api-go/internal/routes/serverdua"
	"mylab-api-go/internal/routes/shared"
)

type Server struct {
	httpServer *http.Server
}

func New(addr string, logLevelRaw string, sqlDB *sql.DB) *Server {
	mux := http.NewServeMux()
	metrics := observability.NewMetrics()
	level := shared.ParseLogLevel(logLevelRaw)

	authCtrl := authcontroller.NewAuthController(sqlDB)
	queryCtrl := querycontroller.NewQueryController(sqlDB)
	crudCtrl := crudcontroller.NewTableCRUDController(sqlDB)
	plgProxy := pluginscontroller.NewPluginProxyController()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		report, status := plgProxy.AggregatePluginsHealth(r.Context())
		shared.WriteJSON(w, status, report)
	})

	// Strict plugin health: returns 503 if any plugin is unhealthy.
	mux.HandleFunc("/healthz/plugins", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		report, status := plgProxy.AggregatePluginsHealthStrict(r.Context())
		shared.WriteJSON(w, status, report)
	})

	// Alias endpoint (typo-friendly)
	mux.HandleFunc("/healthza", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		report, status := plgProxy.AggregatePluginsHealth(r.Context())
		shared.WriteJSON(w, status, report)
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
		if dbURL == "" {
			shared.WriteOK(w, "ready")
			return
		}

		if err := canDialDatabase(dbURL, 2*time.Second); err != nil {
			shared.WriteError(w, http.StatusServiceUnavailable, "Not ready.", map[string]string{"database": err.Error()})
			return
		}

		shared.WriteOK(w, "ready")
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

	// Routes v1
	mux.HandleFunc("/v1/auth/login", authCtrl.HandleLogin)
	mux.HandleFunc("/v1/auth/logout", authCtrl.HandleLogout)
	mux.HandleFunc("/v1/query", queryCtrl.HandleQuery)
	mux.Handle("/v1/crud/", shared.WithRateLimit(http.HandlerFunc(crudCtrl.Handle)))
	mux.Handle("/v1/plugins/", plgProxy)

	// Register route tambahan dari serverdua.go
	serverdua.RegisterRoutesDua(mux)

	srv := &http.Server{
		Addr: addr,
		Handler: shared.WithRecovery(
			shared.WithRequestID(
				shared.WithCORS(
					auth.WithAuth(
						shared.WithAccessLog(level,
							shared.WithMetrics(metrics, mux),
						),
					),
				),
			),
		),
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
