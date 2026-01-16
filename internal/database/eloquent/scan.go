package eloquent

import (
	"database/sql"
)

func scanCurrentRowToMap(rows *sql.Rows) (map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}

	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}

	out := make(map[string]any, len(cols))
	for i, c := range cols {
		out[c] = values[i]
	}
	return out, nil
}
