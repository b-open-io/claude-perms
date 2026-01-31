package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// ApplyResult holds details about what was written
type ApplyResult struct {
	FilePath   string
	Permission string
	LineNumber int  // Line where the permission was added
	WasNew     bool // False if already existed (idempotent)
}

// WritePermissionToUserSettings adds a permission to user settings
func WritePermissionToUserSettings(permission string) (*ApplyResult, error) {
	path := filepath.Join(claudeDir(), "settings.local.json")
	return writePermissionToSettings(path, permission)
}

// WritePermissionToProjectSettings adds a permission to project settings
func WritePermissionToProjectSettings(projectPath, permission string) (*ApplyResult, error) {
	path := filepath.Join(projectPath, ".claude", "settings.local.json")
	return writePermissionToSettings(path, permission)
}

// writePermissionToSettings reads, merges, and writes back settings
func writePermissionToSettings(path, permission string) (*ApplyResult, error) {
	var settings types.Settings

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// File doesn't exist - will create new
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return nil, fmt.Errorf("parse settings: %w", err)
		}
	}

	// Check if already exists (idempotent)
	for _, existing := range settings.Permissions.Allow {
		if existing == permission {
			return &ApplyResult{
				FilePath:   path,
				Permission: permission,
				WasNew:     false,
			}, nil
		}
	}

	// Append new permission
	settings.Permissions.Allow = append(settings.Permissions.Allow, permission)

	// Ensure deny is an empty array, not null (Claude Code rejects null)
	if settings.Permissions.Deny == nil {
		settings.Permissions.Deny = []string{}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Write with pretty formatting
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		return nil, err
	}

	// Find the line number of the permission we just wrote
	lineNumber := findPermissionLine(output, permission)

	return &ApplyResult{
		FilePath:   path,
		Permission: permission,
		LineNumber: lineNumber,
		WasNew:     true,
	}, nil
}

// findPermissionLine scans formatted JSON output for the line containing the permission string
func findPermissionLine(output []byte, permission string) int {
	lines := strings.Split(string(output), "\n")
	target := fmt.Sprintf("%q", permission)
	for i, line := range lines {
		if strings.Contains(line, target) {
			return i + 1 // 1-based
		}
	}
	return 0
}

// DiffLine represents a single line of a diff preview
type DiffLine struct {
	Number int    // Line number in the new file (0 for removed lines)
	Text   string // Line content
	Status rune   // ' ' = context, '+' = added, '-' = removed
}

// PreviewPermissionDiff generates a diff preview for adding permissions to a settings file.
// Returns the file path, the diff lines, and whether the permissions already exist.
func PreviewPermissionDiff(settingsPath string, permissions []string) ([]DiffLine, bool) {
	var settings types.Settings

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	// Check which permissions are new
	existing := make(map[string]bool)
	for _, e := range settings.Permissions.Allow {
		existing[e] = true
	}

	var newPerms []string
	for _, p := range permissions {
		if !existing[p] {
			newPerms = append(newPerms, p)
		}
	}

	if len(newPerms) == 0 {
		// All already exist - show current file state
		if len(data) == 0 {
			return nil, true
		}
		oldLines := strings.Split(string(data), "\n")
		var diff []DiffLine
		for i, line := range oldLines {
			diff = append(diff, DiffLine{Number: i + 1, Text: line, Status: ' '})
		}
		return diff, true
	}

	// Ensure deny is an empty array, not null (Claude Code rejects null)
	if settings.Permissions.Deny == nil {
		settings.Permissions.Deny = []string{}
	}

	// Generate old formatted output
	oldOutput, _ := json.MarshalIndent(settings, "", "  ")
	oldLines := strings.Split(string(oldOutput), "\n")

	// Generate new formatted output
	for _, p := range newPerms {
		settings.Permissions.Allow = append(settings.Permissions.Allow, p)
	}
	newOutput, _ := json.MarshalIndent(settings, "", "  ")
	newLines := strings.Split(string(newOutput), "\n")

	// Build a contextual diff: show a window around the changed lines
	return buildContextDiff(oldLines, newLines), false
}

// PreviewUserDiff previews adding permissions to user settings
func PreviewUserDiff(permissions []string) (string, []DiffLine, bool) {
	path := filepath.Join(claudeDir(), "settings.local.json")
	diff, allExist := PreviewPermissionDiff(path, permissions)
	return path, diff, allExist
}

// PreviewProjectDiff previews adding permissions to a project's settings
func PreviewProjectDiff(projectPath string, permissions []string) (string, []DiffLine, bool) {
	path := filepath.Join(projectPath, ".claude", "settings.local.json")
	diff, allExist := PreviewPermissionDiff(path, permissions)
	return path, diff, allExist
}

// buildContextDiff compares old and new line slices and produces a unified-style diff
// with context lines around changes.
func buildContextDiff(oldLines, newLines []string) []DiffLine {
	// Find the first line that differs
	commonPrefix := 0
	for commonPrefix < len(oldLines) && commonPrefix < len(newLines) && oldLines[commonPrefix] == newLines[commonPrefix] {
		commonPrefix++
	}

	// Find the last line that differs (from the end)
	commonSuffix := 0
	for commonSuffix < len(oldLines)-commonPrefix && commonSuffix < len(newLines)-commonPrefix &&
		oldLines[len(oldLines)-1-commonSuffix] == newLines[len(newLines)-1-commonSuffix] {
		commonSuffix++
	}

	const contextLines = 2

	// Start context: show up to contextLines before the change
	startCtx := commonPrefix - contextLines
	if startCtx < 0 {
		startCtx = 0
	}

	// End context: show up to contextLines after the change
	endOldCtx := len(oldLines) - commonSuffix + contextLines
	if endOldCtx > len(oldLines) {
		endOldCtx = len(oldLines)
	}
	endNewCtx := len(newLines) - commonSuffix + contextLines
	if endNewCtx > len(newLines) {
		endNewCtx = len(newLines)
	}

	var diff []DiffLine

	// Leading ellipsis
	if startCtx > 0 {
		diff = append(diff, DiffLine{Number: 0, Text: "...", Status: ' '})
	}

	// Context before change
	for i := startCtx; i < commonPrefix; i++ {
		diff = append(diff, DiffLine{Number: i + 1, Text: oldLines[i], Status: ' '})
	}

	// Removed lines (from old)
	for i := commonPrefix; i < len(oldLines)-commonSuffix; i++ {
		diff = append(diff, DiffLine{Number: i + 1, Text: oldLines[i], Status: '-'})
	}

	// Added lines (from new)
	for i := commonPrefix; i < len(newLines)-commonSuffix; i++ {
		diff = append(diff, DiffLine{Number: i + 1, Text: newLines[i], Status: '+'})
	}

	// Context after change
	afterStart := len(newLines) - commonSuffix
	for i := afterStart; i < endNewCtx; i++ {
		diff = append(diff, DiffLine{Number: i + 1, Text: newLines[i], Status: ' '})
	}

	// Trailing ellipsis
	if endNewCtx < len(newLines) {
		diff = append(diff, DiffLine{Number: 0, Text: "...", Status: ' '})
	}

	return diff
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
