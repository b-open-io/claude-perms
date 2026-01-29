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

// AgentEntry represents a JSONL entry with agent-specific fields
type AgentEntry struct {
	Type      string          `json:"type"`
	Message   json.RawMessage `json:"message"`
	Timestamp string          `json:"timestamp"`
	Slug      string          `json:"slug"`
	AgentID   string          `json:"agentId"`
}

// TaskInput represents the input structure for Task tool_use
type TaskInput struct {
	SubagentType string `json:"subagent_type"`
	Prompt       string `json:"prompt"`
	Description  string `json:"description"`
}

// LoadAgentUsageStats loads permission usage stats grouped by agent type
func LoadAgentUsageStats(progress chan<- string) ([]types.AgentUsageStats, error) {
	return LoadAgentUsageStatsFrom(filepath.Join(claudeDir(), "projects"), progress)
}

// LoadAgentUsageStatsFrom loads agent usage stats from a specific projects directory
func LoadAgentUsageStatsFrom(projectsDir string, progress chan<- string) ([]types.AgentUsageStats, error) {
	// Walk project directories
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Use the unified cache from cache.go
	cache := loadCache()
	cacheDirty := false

	// Maps for aggregation
	agentIdToAgentType := make(map[string]string)
	agentStats := make(map[string]*agentStatsBuilder)

	// First pass: scan all non-agent session files to build agentId->agentType mapping
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, entry.Name())

		allFiles, err := filepath.Glob(filepath.Join(projectPath, "*.jsonl"))
		if err != nil {
			continue
		}

		projectName := decodeProjectPath(entry.Name())
		if progress != nil {
			progress <- projectName
		}

		for _, sessionFile := range allFiles {
			baseName := filepath.Base(sessionFile)
			if strings.HasPrefix(baseName, "agent-") {
				continue
			}

			// Try cache first
			if cached, hit := getCachedAgentMappings(cache, sessionFile); hit {
				for k, v := range cached {
					agentIdToAgentType[k] = v
				}
				continue
			}

			if progress != nil {
				sessionID := strings.TrimSuffix(baseName, ".jsonl")
				if len(sessionID) > 12 {
					sessionID = sessionID[:12] + "..."
				}
				progress <- "session:" + sessionID
			}

			// Parse and cache
			localMap := make(map[string]string)
			extractAgentIdMappings(sessionFile, localMap)
			for k, v := range localMap {
				agentIdToAgentType[k] = v
			}
			setCachedAgentMappings(cache, sessionFile, localMap)
			cacheDirty = true
		}
	}

	// Second pass: scan agent-*.jsonl files to extract tool_uses
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, entry.Name())
		projectName := decodeProjectPath(entry.Name())

		if progress != nil {
			progress <- projectName
		}

		// Find agent-*.jsonl files at project root
		agentFiles, err := filepath.Glob(filepath.Join(projectPath, "agent-*.jsonl"))
		if err != nil {
			continue
		}

		// Also find agent files in session subagent directories
		subagentFiles, err := filepath.Glob(filepath.Join(projectPath, "*/subagents/agent-*.jsonl"))
		if err == nil {
			agentFiles = append(agentFiles, subagentFiles...)
		}

		for _, agentFile := range agentFiles {
			baseName := filepath.Base(agentFile)
			agentId := strings.TrimSuffix(strings.TrimPrefix(baseName, "agent-"), ".jsonl")

			// Try cache first
			var perms []types.PermissionStats
			var sessionTime time.Time

			if cachedPerms, cachedTime, hit := getCachedAgentSession(cache, agentFile); hit {
				perms = cachedPerms
				sessionTime = cachedTime
			} else {
				perms, sessionTime = parseAgentSession(agentFile)
				setCachedAgentSession(cache, agentFile, perms, sessionTime)
				cacheDirty = true
			}

			agentType, ok := agentIdToAgentType[agentId]
			if !ok {
				agentType = "Unknown"
			}

			if _, exists := agentStats[agentType]; !exists {
				agentStats[agentType] = &agentStatsBuilder{
					agentType:   agentType,
					permissions: make(map[string]*types.PermissionStats),
					sessions:    make(map[string]bool),
					projects:    make(map[string]bool),
					lastSeen:    time.Time{},
				}
			}

			builder := agentStats[agentType]
			builder.sessions[agentFile] = true
			builder.projects[projectName] = true

			for _, p := range perms {
				key := PermissionKey(p.Permission)
				if _, exists := builder.permissions[key]; !exists {
					builder.permissions[key] = &types.PermissionStats{
						Permission: p.Permission,
						Count:      0,
						LastSeen:   time.Time{},
					}
				}
				builder.permissions[key].Count += p.Count
				if p.LastSeen.After(builder.permissions[key].LastSeen) {
					builder.permissions[key].LastSeen = p.LastSeen
				}
			}

			if sessionTime.After(builder.lastSeen) {
				builder.lastSeen = sessionTime
			}
		}
	}

	// Save unified cache if anything changed
	if cacheDirty {
		_ = saveCache(cache)
	}

	// Convert to output slice
	result := make([]types.AgentUsageStats, 0, len(agentStats))
	for agentType, builder := range agentStats {
		perms := make([]types.PermissionStats, 0, len(builder.permissions))
		totalCalls := 0
		for _, p := range builder.permissions {
			perms = append(perms, *p)
			totalCalls += p.Count
		}

		sort.Slice(perms, func(i, j int) bool {
			return perms[i].Count > perms[j].Count
		})

		projects := make([]string, 0, len(builder.projects))
		for proj := range builder.projects {
			projects = append(projects, proj)
		}
		sort.Strings(projects)

		result = append(result, types.AgentUsageStats{
			AgentType:   agentType,
			Permissions: perms,
			TotalCalls:  totalCalls,
			LastSeen:    builder.lastSeen,
			Sessions:    len(builder.sessions),
			Projects:    projects,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCalls > result[j].TotalCalls
	})

	return result, nil
}

// agentStatsBuilder accumulates stats for a single agent type
type agentStatsBuilder struct {
	agentType   string
	permissions map[string]*types.PermissionStats
	sessions    map[string]bool
	projects    map[string]bool
	lastSeen    time.Time
}

// extractAgentIdMappings scans a session file for Task tool_uses and extracts agentId->agentType mappings
func extractAgentIdMappings(sessionPath string, agentIdToAgentType map[string]string) {
	file, err := os.Open(sessionPath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Track pending Task tool_uses: tool_use_id -> agentType
	pendingTasks := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Quick check for relevant content
		hasTask := strings.Contains(string(line), `"Task"`)
		hasSubagent := strings.Contains(string(line), `"subagent_type"`)
		hasToolResult := strings.Contains(string(line), `"tool_result"`)
		hasToolUseResult := strings.Contains(string(line), `"toolUseResult"`)

		if !hasTask && !hasSubagent && !hasToolResult && !hasToolUseResult {
			continue
		}

		// Parse entry with toolUseResult field
		var rawEntry struct {
			Type          string          `json:"type"`
			Message       json.RawMessage `json:"message"`
			ToolUseResult struct {
				AgentID string `json:"agentId"`
			} `json:"toolUseResult"`
		}
		if err := json.Unmarshal(line, &rawEntry); err != nil {
			continue
		}

		// If this is an assistant message with Task tool_use, extract tool_use_id and subagent_type
		if rawEntry.Type == "assistant" && hasTask && hasSubagent {
			// Parse the full message structure to get tool_use ID and subagent_type
			type ToolUseItem struct {
				Type  string    `json:"type"`
				ID    string    `json:"id"`
				Name  string    `json:"name"`
				Input TaskInput `json:"input"`
			}
			type FullMessage struct {
				Content []ToolUseItem `json:"content"`
			}
			var fullMsg FullMessage
			if err := json.Unmarshal(rawEntry.Message, &fullMsg); err != nil {
				continue
			}

			for _, item := range fullMsg.Content {
				if item.Type == "tool_use" && item.Name == "Task" && item.Input.SubagentType != "" {
					pendingTasks[item.ID] = item.Input.SubagentType
				}
			}
		}

		// If user message with tool_result AND toolUseResult.agentId, map agentId to agent type
		if rawEntry.Type == "user" && hasToolResult && rawEntry.ToolUseResult.AgentID != "" {
			// Parse to find the tool_use_id this result is for
			type ToolResultItem struct {
				Type      string `json:"type"`
				ToolUseID string `json:"tool_use_id"`
			}
			type UserMessage struct {
				Content []ToolResultItem `json:"content"`
			}
			var userMsg UserMessage
			if err := json.Unmarshal(rawEntry.Message, &userMsg); err != nil {
				continue
			}

			for _, item := range userMsg.Content {
				if item.Type == "tool_result" {
					if agentType, ok := pendingTasks[item.ToolUseID]; ok {
						agentIdToAgentType[rawEntry.ToolUseResult.AgentID] = agentType
						delete(pendingTasks, item.ToolUseID)
					}
				}
			}
		}
	}
}

// parseAgentSession parses an agent-*.jsonl file and extracts tool_uses
func parseAgentSession(agentPath string) (perms []types.PermissionStats, lastSeen time.Time) {
	file, err := os.Open(agentPath)
	if err != nil {
		return nil, time.Time{}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	counts := make(map[string]int)
	lastSeenMap := make(map[string]time.Time)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Quick check for tool_use
		if !strings.Contains(string(line), `"tool_use"`) {
			continue
		}

		var entry AgentEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Only process assistant messages
		if entry.Type != "assistant" {
			continue
		}

		// Parse message
		var msg AssistantMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		// Extract tool_uses
		for _, item := range msg.Content {
			if item.Type == "tool_use" && item.Name != "" {
				// Skip Task tool itself - we want to track what the agent uses
				if item.Name == "Task" {
					continue
				}

				// Extract full permission with scope
				permString := ExtractPermissionScope(item.Name, item.Input)
				perm := ParsePermission(permString)
				key := PermissionKey(perm)

				counts[key]++

				// Parse entry timestamp
				entryTime := time.Time{}
				if entry.Timestamp != "" {
					if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
						entryTime = t
					}
				}

				if _, exists := lastSeenMap[key]; !exists || entryTime.After(lastSeenMap[key]) {
					lastSeenMap[key] = entryTime
				}

				if entryTime.After(lastSeen) {
					lastSeen = entryTime
				}
			}
		}
	}

	// Convert to stats
	perms = make([]types.PermissionStats, 0, len(counts))
	for key, count := range counts {
		perm := ParsePermission(key)
		perms = append(perms, types.PermissionStats{
			Permission: perm,
			Count:      count,
			LastSeen:   lastSeenMap[key],
		})
	}

	return perms, lastSeen
}
