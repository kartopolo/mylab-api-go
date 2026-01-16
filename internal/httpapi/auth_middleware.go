package httpapi

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

// withAuth enforces that every /v1/* request is associated with a valid user.
// It loads user's company_id and role from DB and stores it in request context.
//
// Current auth transport is intentionally simple:
// - Header: X-User-Id: <int>
func withAuth(sqlDB *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		if sqlDB == nil {
			writeError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
			return
		}

		rawUserID := strings.TrimSpace(r.Header.Get("X-User-Id"))
		if rawUserID == "" {
			writeError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"user_id": "missing X-User-Id header"})
			return
		}
		userID, err := strconv.ParseInt(rawUserID, 10, 64)
		if err != nil || userID <= 0 {
			writeError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"user_id": "invalid"})
			return
		}

		var companyID int64
		var role sql.NullString
		err = sqlDB.QueryRowContext(
			r.Context(),
			"select company_id, role from users where id = $1 limit 1",
			userID,
		).Scan(&companyID, &role)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"user_id": "not found"})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal server error.", nil)
			return
		}
		if companyID <= 0 {
			writeError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"company_id": "invalid"})
			return
		}

		info := AuthInfo{UserID: userID, CompanyID: companyID}
		if role.Valid {
			info.Role = strings.TrimSpace(role.String)
		}

		r = r.WithContext(withAuthInfoInContext(r.Context(), info))
		next.ServeHTTP(w, r)
	})
}
