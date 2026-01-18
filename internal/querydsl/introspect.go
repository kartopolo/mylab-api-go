package querydsl

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"
)

type columnQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type cachedColumns struct {
	cols     map[string]bool
	expires  time.Time
	hasValue bool
}

var columnsCache sync.Map

func loadTableColumns(ctx context.Context, q columnQuerier, table string) (map[string]bool, error) {
	table = strings.ToLower(strings.TrimSpace(table))
	if table == "" {
		return map[string]bool{}, nil
	}

	if v, ok := columnsCache.Load(table); ok {
		cc := v.(cachedColumns)
		if cc.hasValue && time.Now().Before(cc.expires) {
			return cc.cols, nil
		}
	}

	// Best-effort portable enough for Postgres/MySQL; we already use $n placeholders project-wide.
	rows, err := q.QueryContext(ctx,
		"SELECT column_name FROM information_schema.columns WHERE table_name = $1 AND table_schema NOT IN ('pg_catalog','information_schema')",
		table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols[strings.ToLower(strings.TrimSpace(c))] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	columnsCache.Store(table, cachedColumns{cols: cols, expires: time.Now().Add(5 * time.Minute), hasValue: true})
	return cols, nil
}
