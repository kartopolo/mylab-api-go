package shared

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"mylab-api-go/internal/observability"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

type statusCapturingResponseWriter struct {
	w      http.ResponseWriter
	status int
	bytes  int
}

func (s *statusCapturingResponseWriter) Header() http.Header {
	return s.w.Header()
}

func (s *statusCapturingResponseWriter) WriteHeader(code int) {
	s.status = code
	s.w.WriteHeader(code)
}

func (s *statusCapturingResponseWriter) Write(p []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.w.Write(p)
	s.bytes += n
	return n, err
}

func WithRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf(`{"ts":%q,"level":"error","msg":"panic recovered"}`, time.Now().UTC().Format(time.RFC3339Nano))
				err := map[string]string{"code": "panic"}
				rid := RequestIDFromContext(r.Context())
				if rid != "" {
					err["request_id"] = rid
				}
				WriteError(w, http.StatusInternalServerError, "Internal server error.", err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if rid == "" {
			rid = newRequestID()
		}
		w.Header().Set("X-Request-Id", rid)
		r = r.WithContext(WithRequestIDInContext(r.Context(), rid))
		next.ServeHTTP(w, r)
	})
}

func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowedOrigin := corsAllowedOrigin(origin)
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-User-Id,X-Request-Id")
			w.Header().Set("Access-Control-Max-Age", "600")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func corsAllowedOrigin(origin string) string {
	if origin == "" {
		return ""
	}

	// Comma-separated origins, supports "*".
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		// Default: allow common localhost dev origins.
		if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			return origin
		}
		return ""
	}

	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if p == "*" {
			return "*"
		}
		if strings.EqualFold(p, origin) {
			return origin
		}
	}

	return ""
}

func WithAccessLog(level LogLevel, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if level > LogLevelInfo {
			next.ServeHTTP(w, r)
			return
		}

		if shouldSkipAccessLog(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sw := &statusCapturingResponseWriter{w: w}
		next.ServeHTTP(sw, r)

		dur := time.Since(start)
		rid := RequestIDFromContext(r.Context())

		log.Printf(
			`{"ts":%q,"level":"info","request_id":%q,"method":%q,"path":%q,"status":%d,"bytes":%d,"duration_ms":%d}`,
			time.Now().UTC().Format(time.RFC3339Nano),
			rid,
			r.Method,
			r.URL.Path,
			sw.status,
			sw.bytes,
			dur.Milliseconds(),
		)
	})
}

func WithMetrics(m *observability.Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldSkipMetrics(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sw := &statusCapturingResponseWriter{w: w}
		next.ServeHTTP(sw, r)
		m.Observe(r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ParseLogLevel(raw string) LogLevel {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "debug":
		return LogLevelDebug
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "info", "":
		fallthrough
	default:
		return LogLevelInfo
	}
}

func shouldSkipAccessLog(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}

func shouldSkipMetrics(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}
