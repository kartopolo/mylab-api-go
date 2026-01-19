package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Token revocation is kept in-memory (per-process) with expiry.
// This supports immediate logout for the current server instance.
// NOTE: if the service restarts, revoked tokens will be accepted again until they expire.

type revokedEntry struct {
	expUnix int64
}

var revokedTokens = struct {
	mu sync.RWMutex
	m  map[string]revokedEntry
}{m: map[string]revokedEntry{}}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RevokeToken marks a token as revoked until expUnix.
func RevokeToken(token string, expUnix int64) {
	if token == "" {
		return
	}
	if expUnix <= 0 {
		// If exp is unknown, revoke for a short duration.
		expUnix = time.Now().Add(15 * time.Minute).Unix()
	}

	key := tokenHash(token)
	revokedTokens.mu.Lock()
	revokedTokens.m[key] = revokedEntry{expUnix: expUnix}
	revokedTokens.mu.Unlock()
}

// IsTokenRevoked returns true if token was revoked and not yet expired.
func IsTokenRevoked(token string) bool {
	if token == "" {
		return false
	}
	key := tokenHash(token)
	now := time.Now().Unix()

	revokedTokens.mu.RLock()
	entry, ok := revokedTokens.m[key]
	revokedTokens.mu.RUnlock()
	if !ok {
		return false
	}

	// Lazy cleanup
	if entry.expUnix <= now {
		revokedTokens.mu.Lock()
		delete(revokedTokens.m, key)
		revokedTokens.mu.Unlock()
		return false
	}

	return true
}
