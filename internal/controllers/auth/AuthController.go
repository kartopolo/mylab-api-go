package authcontroller

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"mylab-api-go/internal/config"
	"mylab-api-go/internal/routes/shared"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthController struct {
	sqlDB *sql.DB
}

// NewAuthController: inisialisasi controller auth dengan koneksi database.
func NewAuthController(sqlDB *sql.DB) *AuthController {
	return &AuthController{sqlDB: sqlDB}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// HandleLogin: proses POST /v1/auth/login.
// Langkah utama:
// - Validasi method dan JSON body (email, password)
// - Ambil user berdasarkan email, verifikasi bcrypt password
// - Buat JWT dengan klaim user_id, company_id, role, expiry
// - Kembalikan token dan metadata expiry dalam envelope standar
func (c *AuthController) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if c.sqlDB == nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"database": "not configured"})
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req loginRequest
	if err := dec.Decode(&req); err != nil {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", map[string]string{"body": "invalid JSON"})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	validationErrors := map[string]string{}
	if req.Email == "" {
		validationErrors["email"] = "required"
	}
	if req.Password == "" {
		validationErrors["password"] = "required"
	}
	if len(validationErrors) > 0 {
		shared.WriteError(w, http.StatusUnprocessableEntity, "Validation failed.", validationErrors)
		return
	}

	var (
		userID    int64
		companyID int64
		role      sql.NullString
		pwHash    sql.NullString
	)

	// NOTE: This expects Laravel-compatible bcrypt hashes in users.password.
	err := c.sqlDB.QueryRowContext(
		r.Context(),
		"select id, company_id, role, password from users where lower(email) = lower($1) limit 1",
		req.Email,
	).Scan(&userID, &companyID, &role, &pwHash)

	if err == sql.ErrNoRows {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"credentials": "invalid"})
		return
	}
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", nil)
		return
	}
	if userID <= 0 || companyID <= 0 {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"credentials": "invalid"})
		return
	}
	if !pwHash.Valid || strings.TrimSpace(pwHash.String) == "" {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"credentials": "invalid"})
		return
	}

	normalizedHash := strings.TrimSpace(pwHash.String)
	// Laravel/PHP bcrypt sometimes uses $2y$ prefix; Go bcrypt expects $2a$/$2b$.
	if strings.HasPrefix(normalizedHash, "$2y$") {
		normalizedHash = "$2a$" + strings.TrimPrefix(normalizedHash, "$2y$")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(normalizedHash), []byte(req.Password)); err != nil {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"credentials": "invalid"})
		return
	}

	cfg, err := config.Load()
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"config": err.Error()})
		return
	}

	expiry := cfg.JWTExpiry
	if expiry <= 0 {
		expiry = 86400
	}

	roleStr := ""
	if role.Valid {
		roleStr = strings.TrimSpace(role.String)
	}

	expUnix := time.Now().Add(time.Duration(expiry) * time.Second).Unix()
	claims := jwt.MapClaims{
		"user_id":    userID,
		"company_id": companyID,
		"role":       roleStr,
		"exp":        expUnix,
		"iat":        time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", nil)
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"message":    "Login successful.",
		"token":      tokenString,
		"expires_in": expiry,
		"expires_at": expUnix,
		"user_id":    userID,
		"company_id": companyID,
		"role":       roleStr,
	})
}
