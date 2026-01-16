package eloquent

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func castValue(casts map[string]CastType, key string, value any) (any, string) {
	if value == nil {
		return nil, ""
	}
	if casts == nil {
		return value, ""
	}

	ct, ok := casts[key]
	if !ok {
		return value, ""
	}

	switch ct {
	case CastString:
		switch v := value.(type) {
		case string:
			return v, ""
		default:
			return fmt.Sprint(v), ""
		}
	case CastInt:
		switch v := value.(type) {
		case int:
			return int64(v), ""
		case int64:
			return v, ""
		case float64:
			return int64(v), ""
		case jsonNumber:
			i, err := v.Int64()
			if err != nil {
				return nil, "must be an integer"
			}
			return i, ""
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, ""
			}
			i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return nil, "must be an integer"
			}
			return i, ""
		default:
			return nil, "must be an integer"
		}
	case CastFloat:
		switch v := value.(type) {
		case float64:
			return v, ""
		case int:
			return float64(v), ""
		case int64:
			return float64(v), ""
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, ""
			}
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, "must be a number"
			}
			return f, ""
		default:
			return nil, "must be a number"
		}
	case CastBool:
		switch v := value.(type) {
		case bool:
			return v, ""
		case string:
			vv := strings.ToLower(strings.TrimSpace(v))
			if vv == "" {
				return nil, ""
			}
			switch vv {
			case "1", "true", "yes", "y":
				return true, ""
			case "0", "false", "no", "n":
				return false, ""
			default:
				return nil, "must be a boolean"
			}
		default:
			return nil, "must be a boolean"
		}
	case CastDateTime:
		switch v := value.(type) {
		case time.Time:
			return v, ""
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, ""
			}
			t, err := parseDateTime(v)
			if err != nil {
				return nil, "must be a datetime"
			}
			return t, ""
		default:
			return nil, "must be a datetime"
		}
	default:
		return value, ""
	}
}

// jsonNumber mirrors encoding/json.Number without importing encoding/json here.
// (So eloquent stays dependency-light.)
// The http layer may decode using encoding/json, and values may come in as json.Number.
// We'll duck-type it using this interface.
type jsonNumber interface {
	Int64() (int64, error)
	String() string
}

func parseDateTime(raw string) (time.Time, error) {
	v := strings.TrimSpace(raw)
	// Common formats used in MyLab DB + APIs
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid datetime")
}
