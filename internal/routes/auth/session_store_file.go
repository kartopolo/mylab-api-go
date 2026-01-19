package auth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileSessionStore struct {
	dir string
}

func NewFileSessionStore(dir string) (SessionStore, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "storage/sessions"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &fileSessionStore{dir: dir}, nil
}

func (s *fileSessionStore) sessionPath(jti string) string {
	// jti berasal dari hex, jadi aman sebagai nama file.
	return filepath.Join(s.dir, jti+".json")
}

func (s *fileSessionStore) Create(ctx context.Context, sess Session) error {
	_ = ctx
	if strings.TrimSpace(sess.JTI) == "" {
		return errors.New("jti is required")
	}
	if sess.ExpiresAtUnix <= 0 {
		return errors.New("expires_at is required")
	}
	if sess.CreatedAtUnix <= 0 {
		sess.CreatedAtUnix = time.Now().Unix()
	}

	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}

	path := s.sessionPath(sess.JTI)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *fileSessionStore) Get(ctx context.Context, jti string) (Session, bool, error) {
	_ = ctx
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return Session{}, false, nil
	}

	b, err := os.ReadFile(s.sessionPath(jti))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Session{}, false, nil
		}
		return Session{}, false, err
	}

	var sess Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return Session{}, false, err
	}
	return sess, true, nil
}

func (s *fileSessionStore) Revoke(ctx context.Context, jti string, revokedAtUnix int64) error {
	_ = ctx
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return errors.New("jti is required")
	}
	if revokedAtUnix <= 0 {
		revokedAtUnix = time.Now().Unix()
	}

	sess, ok, err := s.Get(context.Background(), jti)
	if err != nil {
		return err
	}
	if !ok {
		// Laravel-like: session tidak ada dianggap sudah logout.
		return nil
	}

	sess.RevokedAtUnix = &revokedAtUnix
	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}

	path := s.sessionPath(jti)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *fileSessionStore) Touch(ctx context.Context, jti string, lastSeenAtUnix int64) error {
	_ = ctx
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return nil
	}
	if lastSeenAtUnix <= 0 {
		lastSeenAtUnix = time.Now().Unix()
	}

	sess, ok, err := s.Get(context.Background(), jti)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	sess.LastSeenAtUnix = &lastSeenAtUnix
	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}

	path := s.sessionPath(jti)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
