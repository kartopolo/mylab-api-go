package querydsl

import (
	"fmt"
	"strconv"
	"strings"

	"mylab-api-go/internal/database/eloquent"
)

// ParseLaravelQuery parses a very small, safe subset of Laravel-style query builder chains.
//
// Supported methods (subset):
// - table('table as alias')
// - select('a.col','b.col')
// - join('table as t','t.col','=','a.col')
// - where('a.col','=','value') OR where('a.col','value')
// - orderby('a.col','desc')
// - take(1)
//
// Anything else is rejected.
func ParseLaravelQuery(raw string) (*QuerySpec, error) {
	q := strings.TrimSpace(raw)
	if q == "" {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"laravel_query": "required"}}
	}

	segments := strings.Split(q, "->")
	if len(segments) == 0 {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"laravel_query": "invalid"}}
	}

	spec := &QuerySpec{Limit: 0}
	for i, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		name, args, err := parseCall(seg)
		if err != nil {
			return nil, &eloquent.ValidationError{Errors: map[string]string{"laravel_query": fmt.Sprintf("invalid segment %d", i)}}
		}

		switch strings.ToLower(name) {
		case "table":
			if len(args) != 1 {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"table": "expects 1 argument"}}
			}
			table, alias, err := parseTableAndAlias(asString(args[0]))
			if err != nil {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"table": "invalid"}}
			}
			spec.FromTable = table
			spec.FromAlias = alias
		case "select":
			if len(args) == 0 {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"select": "empty"}}
			}
			cols := make([]ColumnRef, 0, len(args))
			for _, a := range args {
				c, err := parseColumnRef(asString(a))
				if err != nil {
					return nil, &eloquent.ValidationError{Errors: map[string]string{"select": "invalid column"}}
				}
				cols = append(cols, c)
			}
			spec.Select = cols
		case "join":
			if len(args) != 4 {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"join": "expects 4 arguments"}}
			}
			table, alias, err := parseTableAndAlias(asString(args[0]))
			if err != nil {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"join": "invalid table"}}
			}
			left, err := parseColumnRef(asString(args[1]))
			if err != nil {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"join": "invalid left"}}
			}
			op := strings.TrimSpace(asString(args[2]))
			if op != "=" {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"join": "only '=' supported"}}
			}
			right, err := parseColumnRef(asString(args[3]))
			if err != nil {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"join": "invalid right"}}
			}
			spec.Joins = append(spec.Joins, JoinSpec{Table: table, Alias: alias, On: JoinOn{Left: left, Op: op, Right: right}})
		case "where":
			if len(args) != 2 && len(args) != 3 {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"where": "expects 2 or 3 arguments"}}
			}
			left, err := parseColumnRef(asString(args[0]))
			if err != nil {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"where": "invalid field"}}
			}
			op := "="
			var val any
			if len(args) == 2 {
				val = args[1]
			} else {
				op = strings.ToLower(strings.TrimSpace(asString(args[1])))
				val = args[2]
			}
			switch op {
			case "=", "<=", ">=", "<", ">", "like":
				// ok
			default:
				return nil, &eloquent.ValidationError{Errors: map[string]string{"where": "unsupported operator"}}
			}
			spec.Where = append(spec.Where, WhereSpec{Left: left, Op: op, Value: val})
		case "orderby":
			if len(args) != 2 {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"orderby": "expects 2 arguments"}}
			}
			field, err := parseColumnRef(asString(args[0]))
			if err != nil {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"orderby": "invalid field"}}
			}
			dir := strings.ToLower(strings.TrimSpace(asString(args[1])))
			if dir != "asc" && dir != "desc" {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"orderby": "dir must be asc or desc"}}
			}
			spec.OrderBy = append(spec.OrderBy, OrderBySpec{Field: field, Dir: dir})
		case "take":
			if len(args) != 1 {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"take": "expects 1 argument"}}
			}
			n, err := asInt(args[0])
			if err != nil || n <= 0 {
				return nil, &eloquent.ValidationError{Errors: map[string]string{"take": "invalid"}}
			}
			spec.Limit = n
		default:
			return nil, &eloquent.ValidationError{Errors: map[string]string{"laravel_query": fmt.Sprintf("unsupported method: %s", name)}}
		}
	}

	if spec.FromTable == "" {
		return nil, &eloquent.ValidationError{Errors: map[string]string{"table": "required"}}
	}
	if spec.FromAlias == "" {
		spec.FromAlias = spec.FromTable
	}
	return spec, nil
}
func parseCall(seg string) (name string, args []any, err error) {
	open := strings.IndexByte(seg, '(')
	close := strings.LastIndexByte(seg, ')')
	if open <= 0 || close <= open {
		return "", nil, fmt.Errorf("invalid call")
	}
	name = strings.TrimSpace(seg[:open])
	inside := strings.TrimSpace(seg[open+1 : close])
	args, err = parseArgs(inside)
	if err != nil {
		return "", nil, err
	}
	return name, args, nil
}

func parseArgs(s string) ([]any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []any{}, nil
	}

	args := make([]any, 0, 8)
	for len(s) > 0 {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}

		if s[0] == '\'' {
			// single-quoted string
			s = s[1:]
			end := strings.IndexByte(s, '\'')
			if end < 0 {
				return nil, fmt.Errorf("unterminated string")
			}
			val := s[:end]
			args = append(args, val)
			s = s[end+1:]
		} else {
			// number or bare token
			end := strings.IndexByte(s, ',')
			var token string
			if end < 0 {
				token = strings.TrimSpace(s)
				s = ""
			} else {
				token = strings.TrimSpace(s[:end])
				s = s[end+1:]
			}
			if token == "" {
				continue
			}
			if n, err := strconv.Atoi(token); err == nil {
				args = append(args, n)
			} else {
				// accept bare word (e.g., desc) but treat as string
				args = append(args, token)
			}
			continue
		}

		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, ",") {
			s = s[1:]
			continue
		}
		if s != "" {
			// if there is junk between args, reject
			if s[0] != ',' {
				// allow whitespace only
				if strings.TrimSpace(s) != "" {
					return nil, fmt.Errorf("invalid args")
				}
			}
		}
	}
	return args, nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", v)
	}
}

func asInt(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(t))
	default:
		return 0, fmt.Errorf("not int")
	}
}
