package querydsl

import (
	"strings"
)

// TablePolicy controls which DB tables can be queried.
//
// Rules (denylist-only):
// - AllowedRaw is ignored (kept only for backward-compatible function signature).
// - If DeniedRaw is empty, all tables are allowed.
// - DeniedRaw supports "*" meaning deny all tables.
type TablePolicy struct {
	denyAll bool
	denied  map[string]bool
}

func ParseTablePolicy(allowedRaw, deniedRaw string) TablePolicy {
	// allowedRaw is intentionally ignored (denylist-only policy).
	_ = allowedRaw
	deniedRaw = strings.TrimSpace(deniedRaw)

	p := TablePolicy{
		denied: map[string]bool{},
	}

	if deniedRaw != "" {
		for _, part := range strings.Split(deniedRaw, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if name == "*" {
				p.denyAll = true
				continue
			}
			p.denied[strings.ToLower(name)] = true
		}
	}
	return p
}

func (p TablePolicy) Allows(table string) bool {
	name := strings.ToLower(strings.TrimSpace(table))
	if name == "" {
		return false
	}
	if p.denyAll {
		return false
	}
	return !p.denied[name]
}
