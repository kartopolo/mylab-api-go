package db

import (
	"context"
	"database/sql"
)

type TxFunc[T any] func(tx *sql.Tx) (T, error)

func WithTx[T any](ctx context.Context, db *sql.DB, fn TxFunc[T]) (T, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		var zero T
		return zero, err
	}

	out, err := fn(tx)
	if err != nil {
		_ = tx.Rollback()
		var zero T
		return zero, err
	}

	if err := tx.Commit(); err != nil {
		var zero T
		return zero, err
	}

	return out, nil
}
