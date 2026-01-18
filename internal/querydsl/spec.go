package querydsl

import (
	"fmt"
	"strings"
)

type QuerySpec struct {
	FromTable string
	FromAlias string
	Select    []ColumnRef
	Joins     []JoinSpec
	Where     []WhereSpec
	OrderBy   []OrderBySpec
	Limit     int
}

type ColumnRef struct {
	Alias  string
	Column string
}

type JoinSpec struct {
	Table string
	Alias string
	On    JoinOn
}

type JoinOn struct {
	Left  ColumnRef
	Op    string // only "=" allowed
	Right ColumnRef
}

type WhereSpec struct {
	Left  ColumnRef
	Op    string // =, <=, >=, <, >, like
	Value any
}

type OrderBySpec struct {
	Field ColumnRef
	Dir   string // asc|desc
}

func (c ColumnRef) String() string {
	if c.Alias == "" {
		return c.Column
	}
	return c.Alias + "." + c.Column
}

func parseTableAndAlias(raw string) (table string, alias string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", fmt.Errorf("table is empty")
	}
	parts := strings.Fields(s)
	if len(parts) == 1 {
		return parts[0], parts[0], nil
	}
	if len(parts) == 2 {
		// "harga h"
		return parts[0], parts[1], nil
	}
	if len(parts) == 3 && strings.EqualFold(parts[1], "as") {
		return parts[0], parts[2], nil
	}
	return "", "", fmt.Errorf("invalid table alias format")
}

func parseColumnRef(raw string) (ColumnRef, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ColumnRef{}, fmt.Errorf("empty column")
	}
	parts := strings.Split(s, ".")
	if len(parts) == 1 {
		return ColumnRef{Alias: "", Column: strings.TrimSpace(parts[0])}, nil
	}
	if len(parts) == 2 {
		return ColumnRef{Alias: strings.TrimSpace(parts[0]), Column: strings.TrimSpace(parts[1])}, nil
	}
	return ColumnRef{}, fmt.Errorf("invalid column reference")
}
