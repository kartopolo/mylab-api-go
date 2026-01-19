package authcontroller

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"mylab-api-go/internal/config"
	"mylab-api-go/internal/routes/auth"
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
	issuedAt := time.Now().Unix()

	// Session id (jti) untuk perilaku ala Laravel: 1 token = 1 session.
	jti, err := auth.NewJTI()
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", nil)
		return
	}
	claims := jwt.MapClaims{
		"user_id":    userID,
		"company_id": companyID,
		"role":       roleStr,
		"exp":        expUnix,
		"iat":        issuedAt,
		"jti":        jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", nil)
		return
	}

	// Persist session server-side jika session store diaktifkan.
	if store, ok := auth.GetSessionStore(); ok {
		sess := auth.Session{
			JTI:           jti,
			UserID:        userID,
			CompanyID:     companyID,
			Role:          roleStr,
			ExpiresAtUnix: expUnix,
			CreatedAtUnix: issuedAt,
		}
		if err := store.Create(r.Context(), sess); err != nil {
			shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"session": "store unavailable"})
			return
		}
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

// HandleLogout: proses POST /v1/auth/logout.
// - Memerlukan Authorization: Bearer <token>
// - Token akan direvoke (in-memory) sampai exp JWT.
// - UI tetap harus menghapus token lokal (session berakhir di sisi client).
func (c *AuthController) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "missing or invalid Authorization header"})
		return
	}
	tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if tokenString == "" {
		shared.WriteError(w, http.StatusUnauthorized, "Unauthorized.", map[string]string{"token": "missing"})
		return
	}

	cfg, err := config.Load()
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"config": err.Error()})
		return
	}
	secret := strings.TrimSpace(cfg.JWTSecret)
	if secret == "" {
		secret = "my_secret_key"
	}

	parsed, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	var expUnix int64
	var jti string
	if err == nil && parsed != nil {
		if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
			if exp, ok := claims["exp"].(float64); ok {
				expUnix = int64(exp)
			}
			if jtiRaw, ok := claims["jti"].(string); ok {
				jti = strings.TrimSpace(jtiRaw)
			}
		}
	}

	// Revoke session by jti (server-side), lalu revoke token hash (fallback/legacy).
	if store, ok := auth.GetSessionStore(); ok {
		if jti != "" {
			if err := store.Revoke(r.Context(), jti, time.Now().Unix()); err != nil {
				shared.WriteError(w, http.StatusInternalServerError, "Internal server error.", map[string]string{"session": "store unavailable"})
				return
			}
		}
	}

	auth.RevokeToken(tokenString, expUnix)
	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Logout successful.",
	})
}
