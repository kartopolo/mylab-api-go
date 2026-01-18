package querydsl

import (
	"fmt"

	"mylab-api-go/internal/database/eloquent"
)

type Registry struct {
	tables map[string]func() eloquent.Schema
}

func NewRegistry() *Registry {
	return &Registry{tables: map[string]func() eloquent.Schema{}}
}

func (r *Registry) Register(table string, schema func() eloquent.Schema) {
	r.tables[table] = schema
}

func (r *Registry) Schema(table string) (eloquent.Schema, bool) {
	fn, ok := r.tables[table]
	if !ok {
		return eloquent.Schema{}, false
	}
	return fn(), true
}

func (r *Registry) MustSchema(table string) (eloquent.Schema, error) {
	s, ok := r.Schema(table)
	if !ok {
		return eloquent.Schema{}, fmt.Errorf("unknown table")
	}
	return s, nil
}
