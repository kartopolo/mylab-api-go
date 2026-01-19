package auth

import (
	"net/http"
	"os"
	"strings"
	"time"

	"mylab-api-go/internal/routes/shared"

	"github.com/golang-jwt/jwt/v5"
)

// WithAuth enforces JWT-based authentication for /v1/* endpoints.
func WithAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		// Public auth endpoints (no JWT required)
		if r.URL.Path == "/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "missing or invalid Authorization header"})
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if IsTokenRevoked(tokenString) {
			shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "revoked"})
			return
		}

		secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
		if secret == "" {
			shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"config": "JWT_SECRET is not set"})
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "invalid or expired"})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "invalid claims"})
			return
		}
		// Validate expiry
		nowUnix := time.Now().Unix()
		if exp, ok := claims["exp"].(float64); ok {
			if int64(exp) < nowUnix {
				shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "expired"})
				return
			}
		}
		// Extract user info
		info := AuthInfo{}
		if uid, ok := claims["user_id"].(float64); ok {
			info.UserID = int64(uid)
		}
		if cid, ok := claims["company_id"].(float64); ok {
			info.CompanyID = int64(cid)
		}
		if role, ok := claims["role"].(string); ok {
			info.Role = role
		}

		// Session validation (Laravel-like): if token has jti and store is enabled,
		// require active session.
		if store, ok := GetSessionStore(); ok {
			if jtiRaw, ok := claims["jti"].(string); ok {
				jti := strings.TrimSpace(jtiRaw)
				if jti != "" {
					sess, found, err := store.Get(r.Context(), jti)
					if err != nil {
						shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"session": "store unavailable"})
						return
					}
					if !found {
						shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "session not found"})
						return
					}
					if sess.RevokedAtUnix != nil {
						shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "revoked"})
						return
					}
					if sess.ExpiresAtUnix > 0 && sess.ExpiresAtUnix < nowUnix {
						shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "expired"})
						return
					}
					if sess.UserID > 0 && info.UserID > 0 && sess.UserID != info.UserID {
						shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "invalid session"})
						return
					}
					if sess.CompanyID > 0 && info.CompanyID > 0 && sess.CompanyID != info.CompanyID {
						shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "invalid session"})
						return
					}
					// Best-effort update last seen.
					_ = store.Touch(r.Context(), jti, nowUnix)
				}
			}
		}
		r = r.WithContext(WithAuthInfoInContext(r.Context(), info))
		next.ServeHTTP(w, r)
	})
}
