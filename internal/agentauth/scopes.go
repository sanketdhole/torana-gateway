package agentauth

import "strings"

// ScopeCovers checks if parentScope satisfies or grants childScope under prefix wildcard semantics.
// Examples:
// - "*" covers any scope
// - "db:read" covers "db:read"
// - "db:*" covers "db:read", "db:write", "db:query:users"
// - "analytics:*" covers "analytics:query"
func ScopeCovers(parentScope, childScope string) bool {
	parent := strings.TrimSpace(parentScope)
	child := strings.TrimSpace(childScope)

	if parent == "" || child == "" {
		return false
	}
	if parent == "*" {
		return true
	}
	if parent == child {
		return true
	}

	if strings.HasSuffix(parent, ":*") {
		prefix := strings.TrimSuffix(parent, ":*")
		if child == prefix || strings.HasPrefix(child, prefix+":") {
			return true
		}
	} else if strings.HasSuffix(parent, "*") {
		prefix := strings.TrimSuffix(parent, "*")
		if strings.HasPrefix(child, prefix) {
			return true
		}
	}

	return false
}

// ScopesCover verifies that every scope in childScopes is covered by at least one scope in parentScopes.
// Returns (true, "") if all are covered.
// Returns (false, offendingScope) if any child scope represents an unauthorized privilege escalation.
func ScopesCover(parentScopes, childScopes []string) (bool, string) {
	for _, child := range childScopes {
		child = strings.TrimSpace(child)
		if child == "" {
			continue
		}
		covered := false
		for _, parent := range parentScopes {
			if ScopeCovers(parent, child) {
				covered = true
				break
			}
		}
		if !covered {
			return false, child
		}
	}
	return true, ""
}
