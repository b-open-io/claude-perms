package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/b-open-io/claude-perms/internal/types"
)

const (
	lockAcquireTimeout = 5 * time.Second
	lockPollInterval   = 50 * time.Millisecond
	staleLockMaxAge    = 30 * time.Second
)

type settingsDocument struct {
	root        map[string]json.RawMessage
	permissions map[string]json.RawMessage
	allow       []string
	deny        []string
}

func newSettingsDocument() *settingsDocument {
	return &settingsDocument{
		root:        make(map[string]json.RawMessage),
		permissions: make(map[string]json.RawMessage),
	}
}

func parseSettingsDocument(data []byte) (*settingsDocument, error) {
	doc := newSettingsDocument()
	if len(strings.TrimSpace(string(data))) == 0 {
		return doc, nil
	}

	if err := json.Unmarshal(data, &doc.root); err != nil {
		return nil, fmt.Errorf("parse settings document: %w", err)
	}
	if doc.root == nil {
		doc.root = make(map[string]json.RawMessage)
	}

	if rawPerms, ok := doc.root["permissions"]; ok && len(rawPerms) > 0 {
		if err := json.Unmarshal(rawPerms, &doc.permissions); err != nil {
			return nil, fmt.Errorf("parse permissions object: %w", err)
		}
	}
	if doc.permissions == nil {
		doc.permissions = make(map[string]json.RawMessage)
	}

	if rawAllow, ok := doc.permissions["allow"]; ok && len(rawAllow) > 0 {
		if err := json.Unmarshal(rawAllow, &doc.allow); err != nil {
			return nil, fmt.Errorf("parse permissions.allow: %w", err)
		}
	}

	if rawDeny, ok := doc.permissions["deny"]; ok && len(rawDeny) > 0 {
		if err := json.Unmarshal(rawDeny, &doc.deny); err != nil {
			return nil, fmt.Errorf("parse permissions.deny: %w", err)
		}
	}

	return doc, nil
}

func (d *settingsDocument) marshalIndent() ([]byte, error) {
	if d.root == nil {
		d.root = make(map[string]json.RawMessage)
	}
	if d.permissions == nil {
		d.permissions = make(map[string]json.RawMessage)
	}
	if d.deny == nil {
		d.deny = []string{}
	}

	allowJSON, err := json.Marshal(d.allow)
	if err != nil {
		return nil, fmt.Errorf("marshal permissions.allow: %w", err)
	}
	denyJSON, err := json.Marshal(d.deny)
	if err != nil {
		return nil, fmt.Errorf("marshal permissions.deny: %w", err)
	}
	d.permissions["allow"] = allowJSON
	d.permissions["deny"] = denyJSON

	permsJSON, err := json.Marshal(d.permissions)
	if err != nil {
		return nil, fmt.Errorf("marshal permissions object: %w", err)
	}
	d.root["permissions"] = permsJSON

	return json.MarshalIndent(d.root, "", "  ")
}

func (d *settingsDocument) hasPermission(permission string) bool {
	for _, existing := range d.allow {
		if existing == permission {
			return true
		}
	}
	return false
}

func acquireFileLock(lockPath string) (func(), error) {
	deadline := time.Now().Add(lockAcquireTimeout)

	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "pid=%d time=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
			_ = f.Close()
			return func() {
				_ = os.Remove(lockPath)
			}, nil
		}

		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquire lock %s: %w", lockPath, err)
		}

		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleLockMaxAge {
			_ = os.Remove(lockPath)
			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for file lock: %s", lockPath)
		}
		time.Sleep(lockPollInterval)
	}
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	defer cleanup()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace target file: %w", err)
	}

	// Best-effort directory sync to reduce risk of rename loss on crash.
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return nil
}

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

	doc, err := parseSettingsDocument(data)
	if err != nil {
		return nil, err
	}
	return doc.allow, nil
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
	lockPath := path + ".lock"
	releaseLock, err := acquireFileLock(lockPath)
	if err != nil {
		return nil, err
	}
	defer releaseLock()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		data = nil
	}

	doc, err := parseSettingsDocument(data)
	if err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	// Check if already exists (idempotent)
	if doc.hasPermission(permission) {
		return &ApplyResult{
			FilePath:   path,
			Permission: permission,
			WasNew:     false,
		}, nil
	}

	// Append new permission
	doc.allow = append(doc.allow, permission)

	// Write with pretty formatting
	output, err := doc.marshalIndent()
	if err != nil {
		return nil, err
	}

	if err := writeFileAtomic(path, output, 0644); err != nil {
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
func PreviewPermissionDiff(settingsPath string, permissions []string) ([]DiffLine, bool, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}

	doc, err := parseSettingsDocument(data)
	if err != nil {
		return nil, false, err
	}

	// Check which permissions are new
	existing := make(map[string]bool)
	for _, e := range doc.allow {
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
			return nil, true, nil
		}
		oldLines := strings.Split(string(data), "\n")
		var diff []DiffLine
		for i, line := range oldLines {
			diff = append(diff, DiffLine{Number: i + 1, Text: line, Status: ' '})
		}
		return diff, true, nil
	}

	// Generate old formatted output
	oldOutput, err := doc.marshalIndent()
	if err != nil {
		return nil, false, err
	}
	oldLines := strings.Split(string(oldOutput), "\n")

	// Generate new formatted output
	for _, p := range newPerms {
		doc.allow = append(doc.allow, p)
	}
	newOutput, err := doc.marshalIndent()
	if err != nil {
		return nil, false, err
	}
	newLines := strings.Split(string(newOutput), "\n")

	// Build a contextual diff: show a window around the changed lines
	return buildContextDiff(oldLines, newLines), false, nil
}

// PreviewUserDiff previews adding permissions to user settings
func PreviewUserDiff(permissions []string) (string, []DiffLine, bool, error) {
	path := filepath.Join(claudeDir(), "settings.local.json")
	diff, allExist, err := PreviewPermissionDiff(path, permissions)
	return path, diff, allExist, err
}

// PreviewProjectDiff previews adding permissions to a project's settings
func PreviewProjectDiff(projectPath string, permissions []string) (string, []DiffLine, bool, error) {
	path := filepath.Join(projectPath, ".claude", "settings.local.json")
	diff, allExist, err := PreviewPermissionDiff(path, permissions)
	return path, diff, allExist, err
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
