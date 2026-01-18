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

		secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
		if secret == "" {
			secret = "my_secret_key"
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
		if exp, ok := claims["exp"].(float64); ok {
			if int64(exp) < time.Now().Unix() {
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
		r = r.WithContext(WithAuthInfoInContext(r.Context(), info))
		next.ServeHTTP(w, r)
	})
}
