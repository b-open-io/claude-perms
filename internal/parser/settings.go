package parser

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/b-open-io/claude-perms/internal/types"
)

// LoadUserSettings loads permissions from ~/.claude/settings.local.json
func LoadUserSettings() ([]string, error) {
	path := filepath.Join(claudeDir(), "settings.local.json")
	return loadSettingsPermissions(path)
}

// LoadProjectSettings loads permissions from .claude/settings.local.json in project
func LoadProjectSettings(projectPath string) ([]string, error) {
	path := filepath.Join(projectPath, ".claude", "settings.local.json")
	return loadSettingsPermissions(path)
}

// loadSettingsPermissions reads a settings file and returns allowed permissions
func loadSettingsPermissions(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var settings types.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return settings.Permissions.Allow, nil
}

// IsApprovedUser checks if a permission is approved at user level
func IsApprovedUser(perm string, userApproved []string) bool {
	for _, approved := range userApproved {
		if matchesPermission(perm, approved) {
			return true
		}
	}
	return false
}

// IsApprovedProject checks if a permission is approved at project level
func IsApprovedProject(perm string, projectApproved []string) bool {
	for _, approved := range projectApproved {
		if matchesPermission(perm, approved) {
			return true
		}
	}
	return false
}

// matchesPermission checks if a permission matches an approval pattern
// Supports wildcards like "Bash(*)" matching "Bash(curl:*)"
func matchesPermission(perm, pattern string) bool {
	// Exact match
	if perm == pattern {
		return true
	}

	// Parse both
	permParsed := ParsePermission(perm)
	patternParsed := ParsePermission(pattern)

	// Type must match
	if permParsed.Type != patternParsed.Type {
		return false
	}

	// Check scope patterns
	if patternParsed.Scope == "*" || patternParsed.Scope == "" {
		return true
	}

	return permParsed.Scope == patternParsed.Scope
}

// GetApprovalLevel returns the approval level for a permission
func GetApprovalLevel(perm string, userApproved, projectApproved []string) types.ApprovalLevel {
	if IsApprovedUser(perm, userApproved) {
		return types.ApprovedUser
	}
	if IsApprovedProject(perm, projectApproved) {
		return types.ApprovedProject
	}
	return types.NotApproved
}
