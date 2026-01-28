package parser

import (
	"regexp"
	"strings"

	"github.com/b-open-io/claude-perms/internal/types"
)

// permissionPattern matches permissions like "Bash(curl:*)" or "Write"
var permissionPattern = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?$`)

// ParsePermission parses a permission string into a Permission struct
func ParsePermission(raw string) types.Permission {
	raw = strings.TrimSpace(raw)

	matches := permissionPattern.FindStringSubmatch(raw)
	if matches == nil {
		// Fallback for unparseable permissions
		return types.Permission{
			Type:  raw,
			Scope: "",
			Raw:   raw,
		}
	}

	return types.Permission{
		Type:  matches[1],
		Scope: matches[2],
		Raw:   raw,
	}
}

// ParsePermissions parses a slice of permission strings
func ParsePermissions(rawPerms []string) []types.Permission {
	perms := make([]types.Permission, 0, len(rawPerms))
	for _, raw := range rawPerms {
		perms = append(perms, ParsePermission(raw))
	}
	return perms
}

// PermissionKey returns a unique key for a permission (the raw string)
func PermissionKey(p types.Permission) string {
	return p.Raw
}
