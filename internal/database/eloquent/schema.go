package eloquent

import (
	"strings"
	"time"
)

type CastType string

const (
	CastString   CastType = "string"
	CastInt      CastType = "int"
	CastFloat    CastType = "float"
	CastBool     CastType = "bool"
	CastDateTime CastType = "datetime"
)

type Schema struct {
	Table       string
	PrimaryKey  string
	Columns     []string
	Casts       map[string]CastType
	Fillable    []string
	Aliases     map[string]string
	Timestamps  bool
	Now         func() time.Time
}

func (s Schema) withDefaults() Schema {
	out := s
	if out.Now == nil {
		out.Now = time.Now
	}
	return out
}

func (s Schema) hasColumn(col string) bool {
	for _, c := range s.Columns {
		if c == col {
			return true
		}
	}
	return false
}

func (s Schema) fillableSet() map[string]bool {
	set := map[string]bool{}
	if len(s.Fillable) > 0 {
		for _, f := range s.Fillable {
			set[f] = true
		}
		return set
	}

	// Default behavior aligned with mylab-core BaseModel comment:
	// if fillable is empty, allow all columns except PK.
	for _, c := range s.Columns {
		if c == s.PrimaryKey {
			continue
		}
		set[c] = true
	}
	return set
}

func (s Schema) normalizePayload(payload map[string]any) (map[string]any, *ValidationError) {
	s = s.withDefaults()
	data := map[string]any{}
	allowed := s.fillableSet()

	// Apply aliases first (payload keys can map to real DB columns)
	for k, v := range payload {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if s.Aliases != nil {
			if mapped, ok := s.Aliases[key]; ok {
				key = mapped
			}
		}
		data[key] = v
	}

	// Filter to fillable + cast
	out := map[string]any{}
	errs := map[string]string{}
	for k, v := range data {
		if !allowed[k] {
			continue
		}
		casted, err := castValue(s.Casts, k, v)
		if err != "" {
			errs[k] = err
			continue
		}
		out[k] = casted
	}

	if len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}

	return out, nil
}
