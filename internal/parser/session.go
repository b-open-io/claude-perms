package parser

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/b-open-io/claude-perms/internal/types"
)

// claudeDir returns the path to ~/.claude
func claudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// SessionsIndex represents the actual sessions-index.json structure
type SessionsIndex struct {
	Version int            `json:"version"`
	Entries []SessionEntry `json:"entries"`
}

// SessionEntry represents a single session in the index
type SessionEntry struct {
	SessionID string `json:"sessionId"`
	FullPath  string `json:"fullPath"`
	FileMtime int64  `json:"fileMtime"`
	Modified  string `json:"modified"`
}

// LoadAllPermissionStats loads permission stats from all session logs
func LoadAllPermissionStats() ([]types.PermissionStats, error) {
	return LoadAllPermissionStatsFrom(filepath.Join(claudeDir(), "projects"))
}

// LoadAllPermissionStatsFrom loads permission stats from a specific projects directory
func LoadAllPermissionStatsFrom(projectsDir string) ([]types.PermissionStats, error) {
	// Map to aggregate stats by permission
	statsMap := make(map[string]*types.PermissionStats)
	projectsMap := make(map[string]map[string]bool) // permission -> set of projects

	// Walk all project directories
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, entry.Name())
		projectName := decodeProjectPath(entry.Name())

		// Read sessions index
		indexPath := filepath.Join(projectPath, "sessions-index.json")
		sessions, err := loadSessionsIndex(indexPath)
		if err != nil {
			continue // Skip projects without valid index
		}

		// Process each session
		for _, session := range sessions {
			sessionPath := filepath.Join(projectPath, session.SessionID+".jsonl")

			// Parse modified time
			var sessionTime time.Time
			if session.Modified != "" {
				sessionTime, _ = time.Parse(time.RFC3339, session.Modified)
			}
			if sessionTime.IsZero() {
				sessionTime = time.Unix(session.FileMtime/1000, 0)
			}

			perms, err := parseSessionLog(sessionPath, sessionTime)
			if err != nil {
				continue
			}

			// Aggregate stats
			for _, p := range perms {
				key := PermissionKey(p.Permission)

				if _, exists := statsMap[key]; !exists {
					statsMap[key] = &types.PermissionStats{
						Permission: p.Permission,
						Count:      0,
						LastSeen:   time.Time{},
						Projects:   nil,
					}
					projectsMap[key] = make(map[string]bool)
				}

				statsMap[key].Count += p.Count
				if p.LastSeen.After(statsMap[key].LastSeen) {
					statsMap[key].LastSeen = p.LastSeen
				}
				projectsMap[key][projectName] = true
			}
		}
	}

	// Convert map to slice
	stats := make([]types.PermissionStats, 0, len(statsMap))
	for key, s := range statsMap {
		// Build projects list
		projects := make([]string, 0, len(projectsMap[key]))
		for proj := range projectsMap[key] {
			projects = append(projects, proj)
		}
		sort.Strings(projects)
		s.Projects = projects

		stats = append(stats, *s)
	}

	// Sort by count (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	return stats, nil
}

// loadSessionsIndex reads and parses sessions-index.json
func loadSessionsIndex(path string) ([]SessionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var index SessionsIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return index.Entries, nil
}

// JSONLEntry represents a line in the JSONL session log
type JSONLEntry struct {
	Type      string          `json:"type"`
	Message   json.RawMessage `json:"message"`
	Timestamp string          `json:"timestamp"`
}

// AssistantMessage represents the message field for assistant entries
type AssistantMessage struct {
	Role    string        `json:"role"`
	Content []ContentItem `json:"content"`
}

// ContentItem represents an item in the content array
type ContentItem struct {
	Type string `json:"type"`
	Name string `json:"name"` // Tool name for tool_use
}

// parseSessionLog parses a JSONL session log and extracts tool_use events
func parseSessionLog(path string, sessionTime time.Time) ([]types.PermissionStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Map to count permissions in this session
	counts := make(map[string]int)
	lastSeen := make(map[string]time.Time)

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Quick check for tool_use before full parse
		if !strings.Contains(string(line), `"tool_use"`) {
			continue
		}

		var entry JSONLEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Only process assistant messages
		if entry.Type != "assistant" {
			continue
		}

		// Parse the message
		var msg AssistantMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		// Extract tool names from content
		for _, item := range msg.Content {
			if item.Type == "tool_use" && item.Name != "" {
				perm := ParsePermission(item.Name)
				key := PermissionKey(perm)

				counts[key]++

				// Parse entry timestamp
				entryTime := sessionTime
				if entry.Timestamp != "" {
					if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
						entryTime = t
					}
				}

				if _, exists := lastSeen[key]; !exists || entryTime.After(lastSeen[key]) {
					lastSeen[key] = entryTime
				}
			}
		}
	}

	// Convert to stats
	stats := make([]types.PermissionStats, 0, len(counts))
	for key, count := range counts {
		perm := ParsePermission(key)
		stats = append(stats, types.PermissionStats{
			Permission: perm,
			Count:      count,
			LastSeen:   lastSeen[key],
		})
	}

	return stats, nil
}

// decodeProjectPath converts encoded project path back to readable form
// e.g., "-Users-satchmo-code-myproject" -> "/Users/satchmo/code/myproject"
func decodeProjectPath(encoded string) string {
	if strings.HasPrefix(encoded, "-") {
		return "/" + strings.ReplaceAll(encoded[1:], "-", "/")
	}
	return strings.ReplaceAll(encoded, "-", "/")
}
