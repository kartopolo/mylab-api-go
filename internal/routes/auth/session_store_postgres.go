package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type postgresSessionStore struct {
	db    *sql.DB
	table string
}

func NewPostgresSessionStore(db *sql.DB, table string) (SessionStore, error) {
	if db == nil {
		return nil, errors.New("db is required")
	}
	table = strings.TrimSpace(table)
	if table == "" {
		table = "auth_sessions"
	}
	st := &postgresSessionStore{db: db, table: table}
	if err := st.ensureTable(context.Background()); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *postgresSessionStore) ensureTable(ctx context.Context) error {
	// Simpel auto-migration (best-effort) agar bisa jalan tanpa langkah manual.
	// Skema memakai *_unix BIGINT untuk stabil (tanpa isu timezone).
	createTable := fmt.Sprintf(`
create table if not exists %s (
  jti text primary key,
  user_id bigint not null,
  company_id bigint not null,
  role text not null default '',
  expires_at_unix bigint not null,
  created_at_unix bigint not null,
  revoked_at_unix bigint null,
  last_seen_at_unix bigint null
)
`, s.table)
	if _, err := s.db.ExecContext(ctx, createTable); err != nil {
		return err
	}

	idxUser := fmt.Sprintf(`create index if not exists %s_user_id_idx on %s (user_id)`, s.table, s.table)
	if _, err := s.db.ExecContext(ctx, idxUser); err != nil {
		return err
	}

	idxCompany := fmt.Sprintf(`create index if not exists %s_company_id_idx on %s (company_id)`, s.table, s.table)
	if _, err := s.db.ExecContext(ctx, idxCompany); err != nil {
		return err
	}

	return nil
}

func (s *postgresSessionStore) Create(ctx context.Context, sess Session) error {
	if strings.TrimSpace(sess.JTI) == "" {
		return errors.New("jti is required")
	}
	if sess.ExpiresAtUnix <= 0 {
		return errors.New("expires_at is required")
	}
	if sess.CreatedAtUnix <= 0 {
		sess.CreatedAtUnix = time.Now().Unix()
	}
	role := strings.TrimSpace(sess.Role)

	q := fmt.Sprintf(`
insert into %s (jti, user_id, company_id, role, expires_at_unix, created_at_unix, revoked_at_unix, last_seen_at_unix)
values ($1,$2,$3,$4,$5,$6,null,null)
`, s.table)
	_, err := s.db.ExecContext(ctx, q, sess.JTI, sess.UserID, sess.CompanyID, role, sess.ExpiresAtUnix, sess.CreatedAtUnix)
	return err
}

func (s *postgresSessionStore) Get(ctx context.Context, jti string) (Session, bool, error) {
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return Session{}, false, nil
	}

	q := fmt.Sprintf(`
select jti, user_id, company_id, role, expires_at_unix, created_at_unix, revoked_at_unix, last_seen_at_unix
from %s where jti = $1
`, s.table)

	var out Session
	var revoked sql.NullInt64
	var lastSeen sql.NullInt64
	err := s.db.QueryRowContext(ctx, q, jti).Scan(
		&out.JTI,
		&out.UserID,
		&out.CompanyID,
		&out.Role,
		&out.ExpiresAtUnix,
		&out.CreatedAtUnix,
		&revoked,
		&lastSeen,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, false, nil
		}
		return Session{}, false, err
	}
	if revoked.Valid {
		t := revoked.Int64
		out.RevokedAtUnix = &t
	}
	if lastSeen.Valid {
		t := lastSeen.Int64
		out.LastSeenAtUnix = &t
	}
	return out, true, nil
}

func (s *postgresSessionStore) Revoke(ctx context.Context, jti string, revokedAtUnix int64) error {
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return errors.New("jti is required")
	}
	if revokedAtUnix <= 0 {
		revokedAtUnix = time.Now().Unix()
	}
	q := fmt.Sprintf(`update %s set revoked_at_unix = $2 where jti = $1`, s.table)
	_, err := s.db.ExecContext(ctx, q, jti, revokedAtUnix)
	return err
}

func (s *postgresSessionStore) Touch(ctx context.Context, jti string, lastSeenAtUnix int64) error {
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return nil
	}
	if lastSeenAtUnix <= 0 {
		lastSeenAtUnix = time.Now().Unix()
	}
	q := fmt.Sprintf(`update %s set last_seen_at_unix = $2 where jti = $1`, s.table)
	_, err := s.db.ExecContext(ctx, q, jti, lastSeenAtUnix)
	return err
}
