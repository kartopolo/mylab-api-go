package querydsl

import (
	"strings"
)

// TablePolicy controls which DB tables can be queried.
//
// Rules:
// - If AllowedRaw is set (non-empty), it takes precedence and DeniedRaw is ignored.
// - AllowedRaw supports "*" meaning allow all tables.
// - If AllowedRaw is empty, all tables are allowed except those in DeniedRaw.
type TablePolicy struct {
	allowlistMode bool
	allowAll      bool
	allowed       map[string]bool
	denied        map[string]bool
}

func ParseTablePolicy(allowedRaw, deniedRaw string) TablePolicy {
	allowedRaw = strings.TrimSpace(allowedRaw)
	deniedRaw = strings.TrimSpace(deniedRaw)

	p := TablePolicy{
		allowed: map[string]bool{},
		denied:  map[string]bool{},
	}

	if allowedRaw != "" {
		p.allowlistMode = true
		for _, part := range strings.Split(allowedRaw, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if name == "*" {
				p.allowAll = true
				continue
			}
			p.allowed[strings.ToLower(name)] = true
		}
		return p
	}

	// denylist mode (default)
	if deniedRaw != "" {
		for _, part := range strings.Split(deniedRaw, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
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
	if p.allowlistMode {
		if p.allowAll {
			return true
		}
		return p.allowed[name]
	}
	return !p.denied[name]
}
