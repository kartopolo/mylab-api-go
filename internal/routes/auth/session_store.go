package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

// SessionStore adalah penyimpanan state sesi/token server-side.
// Ini dipakai untuk perilaku ala Laravel session: token punya session id (jti)
// dan logout hanya mematikan session tersebut (tidak mempengaruhi device lain).
//
// NOTE:
// - JWT tetap dipakai sebagai credential.
// - Store ini adalah sumber kebenaran apakah session masih aktif/revoked.
// - Jika driver tidak diset, middleware akan berjalan seperti sebelumnya.

type Session struct {
	JTI            string
	UserID         int64
	CompanyID      int64
	Role           string
	ExpiresAtUnix  int64
	CreatedAtUnix  int64
	RevokedAtUnix  *int64
	LastSeenAtUnix *int64
}

type SessionStore interface {
	Create(ctx context.Context, s Session) error
	Get(ctx context.Context, jti string) (Session, bool, error)
	Revoke(ctx context.Context, jti string, revokedAtUnix int64) error
	Touch(ctx context.Context, jti string, lastSeenAtUnix int64) error
}

var sessionStoreHolder = struct {
	mu sync.RWMutex
	s  SessionStore
}{}

func SetSessionStore(store SessionStore) {
	sessionStoreHolder.mu.Lock()
	sessionStoreHolder.s = store
	sessionStoreHolder.mu.Unlock()
}

func GetSessionStore() (SessionStore, bool) {
	sessionStoreHolder.mu.RLock()
	defer sessionStoreHolder.mu.RUnlock()
	if sessionStoreHolder.s == nil {
		return nil, false
	}
	return sessionStoreHolder.s, true
}

// NewJTI generates a random session id for JWT "jti".
func NewJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func normalizeDriver(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "file"
	}
	return s
}

var ErrSessionStoreNotSupported = errors.New("session store driver not supported")
