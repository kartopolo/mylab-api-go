package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"mylab-api-go/internal/observability"
)

type logLevel int

const (
	logLevelDebug logLevel = iota
	logLevelInfo
	logLevelWarn
	logLevelError
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

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf(`{"ts":%q,"level":"error","msg":"panic recovered"}`, time.Now().UTC().Format(time.RFC3339Nano))
				writeError(w, http.StatusInternalServerError, "Internal server error.", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if rid == "" {
			rid = newRequestID()
		}
		w.Header().Set("X-Request-Id", rid)
		r = r.WithContext(withRequestIDInContext(r.Context(), rid))
		next.ServeHTTP(w, r)
	})
}

func withAccessLog(level logLevel, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if level > logLevelInfo {
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
		rid := requestIDFromContext(r.Context())

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

func withMetrics(m *observability.Metrics, next http.Handler) http.Handler {
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

func parseLogLevel(raw string) logLevel {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "debug":
		return logLevelDebug
	case "warn", "warning":
		return logLevelWarn
	case "error":
		return logLevelError
	case "info", "":
		fallthrough
	default:
		return logLevelInfo
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
	// Avoid self-scrape inflation.
	return path == "/metrics"
}
