package shared

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rlEntry struct {
    mu        sync.Mutex
    tokens    float64
    last      time.Time
    lastTouch time.Time
}

type rateLimiter struct {
    mu       sync.Mutex
    m        map[string]*rlEntry
    ratePerS float64
    burst    float64
}

// WithRateLimit provides a simple in-memory per-IP token bucket rate limiter.
// Config via env:
// - RL_RATE_PER_MIN (int, default 60)
// - RL_BURST (int, default 20)
func WithRateLimit(next http.Handler) http.Handler {
    rawRate := stringsTrimOrEnv("RL_RATE_PER_MIN", "60")
    rawBurst := stringsTrimOrEnv("RL_BURST", "20")
    rpm, _ := strconv.Atoi(rawRate)
    burst, _ := strconv.Atoi(rawBurst)

    rl := &rateLimiter{
        m:        map[string]*rlEntry{},
        ratePerS: float64(rpm) / 60.0,
        burst:    float64(burst),
    }

    // cleanup goroutine
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        for range ticker.C {
            rl.mu.Lock()
            now := time.Now()
            for k, e := range rl.m {
                if now.Sub(e.lastTouch) > 5*time.Minute {
                    delete(rl.m, k)
                }
            }
            rl.mu.Unlock()
        }
    }()

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !stringsHasPrefix(r.URL.Path, "/v1/crud/") {
            next.ServeHTTP(w, r)
            return
        }

        ip := remoteIP(r)
        if ip == "" {
            // fail open if we can't determine IP
            next.ServeHTTP(w, r)
            return
        }

        e := rl.getEntry(ip)
        now := time.Now()
        e.mu.Lock()
        // refill
        elapsed := now.Sub(e.last).Seconds()
        e.tokens += elapsed * rl.ratePerS
        if e.tokens > rl.burst {
            e.tokens = rl.burst
        }
        e.last = now
        e.lastTouch = now

        if e.tokens < 1.0 {
            e.mu.Unlock()
            w.Header().Set("Retry-After", "1")
            http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
            return
        }
        e.tokens -= 1.0
        e.mu.Unlock()

        next.ServeHTTP(w, r)
    })
}

func (rl *rateLimiter) getEntry(key string) *rlEntry {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    e, ok := rl.m[key]
    if !ok {
        e = &rlEntry{tokens: rl.burst, last: time.Now(), lastTouch: time.Now()}
        rl.m[key] = e
    }
    return e
}

func remoteIP(r *http.Request) string {
    // Try X-Forwarded-For then RemoteAddr
    if x := r.Header.Get("X-Forwarded-For"); x != "" {
        // first entry
        parts := splitComma(x)
        if len(parts) > 0 {
            return parts[0]
        }
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}

// minimal helpers to avoid extra imports elsewhere
func stringsTrimOrEnv(key, def string) string {
    v := stringsTrimSpace(os.Getenv(key))
    if v == "" {
        return def
    }
    return v
}

func stringsTrimSpace(s string) string { return strings.TrimSpace(s) }
func stringsHasPrefix(s, p string) bool { return strings.HasPrefix(s, p) }
func splitComma(s string) []string { return strings.Split(s, ",") }
