# Matrix View Agent Permission Tracking

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track which permissions each agent actually requests during usage, display in Matrix view with responsive layout, and allow batch approval of selected permissions.

**Architecture:** Parse session logs to correlate Task tool calls (containing `subagent_type`) with subagent sessions (via `slug` field). Aggregate permission usage per agent type. Display in responsive table, with detail modal supporting multi-select and batch approval.

**Tech Stack:** Go, Bubbletea, Lipgloss, existing parser infrastructure

---

## Task 1: Add Agent Usage Types

**Files:**
- Modify: `internal/types/types.go`

**Step 1: Add AgentUsageStats type**

In `internal/types/types.go`, add after SkillPermissions:

```go
// AgentUsageStats tracks actual permission usage by an agent type
type AgentUsageStats struct {
	AgentType   string            // "Explore", "bopen-tools:devops-specialist", etc.
	Permissions []PermissionStats // Actual tool_uses from this agent
	TotalCalls  int               // Sum of all permission counts
	LastSeen    time.Time         // Most recent activity
	Sessions    int               // Number of sessions this agent ran in
	Projects    []string          // Projects where this agent was used
}
```

**Step 2: Build and verify**

Run: `go build ./...`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/types/types.go
git commit -m "feat: add AgentUsageStats type for tracking agent permission usage"
```

---

## Task 2: Create Agent Usage Parser

**Files:**
- Create: `internal/parser/agent_usage.go`

**Step 1: Create the parser file**

Create `internal/parser/agent_usage.go`:

```go
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

// LoadAgentUsageStats loads permission usage stats grouped by agent type
func LoadAgentUsageStats() ([]types.AgentUsageStats, error) {
	return LoadAgentUsageStatsFrom(filepath.Join(claudeDir(), "projects"))
}

// LoadAgentUsageStatsFrom loads agent usage stats from a specific projects directory
func LoadAgentUsageStatsFrom(projectsDir string) ([]types.AgentUsageStats, error) {
	// Map of slug -> agentType (from parent sessions)
	slugToAgent := make(map[string]string)

	// Map of agentType -> aggregated stats
	agentStats := make(map[string]*types.AgentUsageStats)

	// Track projects per agent
	agentProjects := make(map[string]map[string]bool)

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

		// First pass: scan parent sessions for Task calls to build slug->agent map
		files, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			// Skip agent files in first pass
			if strings.HasPrefix(f.Name(), "agent-") {
				continue
			}

			sessionPath := filepath.Join(projectPath, f.Name())
			extractSlugToAgentMap(sessionPath, slugToAgent)
		}

		// Second pass: scan agent sessions for tool_uses
		for _, f := range files {
			if !strings.HasPrefix(f.Name(), "agent-") {
				continue
			}

			agentSessionPath := filepath.Join(projectPath, f.Name())
			slug, perms, lastSeen := parseAgentSession(agentSessionPath)

			if slug == "" || len(perms) == 0 {
				continue
			}

			agentType := slugToAgent[slug]
			if agentType == "" {
				agentType = "unknown"
			}

			// Initialize agent stats if needed
			if _, exists := agentStats[agentType]; !exists {
				agentStats[agentType] = &types.AgentUsageStats{
					AgentType:   agentType,
					Permissions: nil,
					TotalCalls:  0,
					Sessions:    0,
				}
				agentProjects[agentType] = make(map[string]bool)
			}

			stats := agentStats[agentType]
			stats.Sessions++
			agentProjects[agentType][projectName] = true

			if lastSeen.After(stats.LastSeen) {
				stats.LastSeen = lastSeen
			}

			// Merge permissions
			stats.Permissions = mergePermissionStats(stats.Permissions, perms)

			// Update total calls
			for _, p := range perms {
				stats.TotalCalls += p.Count
			}
		}
	}

	// Convert to slice and add projects
	result := make([]types.AgentUsageStats, 0, len(agentStats))
	for agentType, stats := range agentStats {
		projects := make([]string, 0, len(agentProjects[agentType]))
		for proj := range agentProjects[agentType] {
			projects = append(projects, proj)
		}
		sort.Strings(projects)
		stats.Projects = projects
		result = append(result, *stats)
	}

	// Sort by total calls descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCalls > result[j].TotalCalls
	})

	return result, nil
}

// extractSlugToAgentMap scans a parent session for Task calls and extracts slug->agentType mappings
func extractSlugToAgentMap(sessionPath string, slugToAgent map[string]string) {
	file, err := os.Open(sessionPath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Track Task tool_use IDs and their agent types
	taskAgentTypes := make(map[string]string) // tool_use_id -> agentType

	for scanner.Scan() {
		line := scanner.Bytes()

		// Quick filters
		if !strings.Contains(string(line), `"Task"`) && !strings.Contains(string(line), `"tool_result"`) {
			continue
		}

		var entry JSONLEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		for _, item := range msg.Content {
			// Capture Task tool_use with subagent_type
			if item.Type == "tool_use" && item.Name == "Task" {
				var input struct {
					SubagentType string `json:"subagent_type"`
				}
				if err := json.Unmarshal(item.Input, &input); err == nil && input.SubagentType != "" {
					// Get tool_use id from raw JSON
					var rawItem struct {
						ID string `json:"id"`
					}
					// Re-parse to get ID (item.Input doesn't have it)
					taskAgentTypes[rawItem.ID] = input.SubagentType
				}
			}

			// Capture tool_result with slug
			if item.Type == "tool_result" {
				toolUseID := getToolUseID(item)
				if agentType, exists := taskAgentTypes[toolUseID]; exists {
					// Get slug from entry
					var rawEntry struct {
						Slug string `json:"slug"`
					}
					if err := json.Unmarshal(line, &rawEntry); err == nil && rawEntry.Slug != "" {
						slugToAgent[rawEntry.Slug] = agentType
					}
				}
			}
		}

		// Also check entry-level for slug on tool_result entries
		var rawEntry struct {
			Slug string `json:"slug"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &rawEntry); err == nil {
			if rawEntry.Type == "user" && rawEntry.Slug != "" {
				// This is a tool_result entry, check if we have a pending Task
				// The slug appears on the result, need to correlate with the Task call
			}
		}
	}
}

// getToolUseID extracts tool_use_id from a content item
func getToolUseID(item ContentItem) string {
	var raw struct {
		ToolUseID string `json:"tool_use_id"`
	}
	// ContentItem doesn't have tool_use_id directly, need to check raw
	return raw.ToolUseID
}

// parseAgentSession extracts slug and permission stats from an agent session file
func parseAgentSession(sessionPath string) (string, []types.PermissionStats, time.Time) {
	file, err := os.Open(sessionPath)
	if err != nil {
		return "", nil, time.Time{}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var slug string
	var lastSeen time.Time
	counts := make(map[string]int)
	permLastSeen := make(map[string]time.Time)

	for scanner.Scan() {
		line := scanner.Bytes()

		var entry JSONLEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Extract slug from any entry
		if slug == "" {
			var rawEntry struct {
				Slug string `json:"slug"`
			}
			if err := json.Unmarshal(line, &rawEntry); err == nil && rawEntry.Slug != "" {
				slug = rawEntry.Slug
			}
		}

		// Parse timestamp
		var entryTime time.Time
		if entry.Timestamp != "" {
			entryTime, _ = time.Parse(time.RFC3339, entry.Timestamp)
		}
		if entryTime.After(lastSeen) {
			lastSeen = entryTime
		}

		// Only process assistant messages for tool_use
		if entry.Type != "assistant" {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		for _, item := range msg.Content {
			if item.Type == "tool_use" && item.Name != "" {
				permString := ExtractPermissionScope(item.Name, item.Input)
				perm := ParsePermission(permString)
				key := PermissionKey(perm)

				counts[key]++
				if entryTime.After(permLastSeen[key]) {
					permLastSeen[key] = entryTime
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
			LastSeen:   permLastSeen[key],
		})
	}

	// Sort by count
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	return slug, stats, lastSeen
}

// mergePermissionStats merges two permission stat slices
func mergePermissionStats(existing, new []types.PermissionStats) []types.PermissionStats {
	statsMap := make(map[string]*types.PermissionStats)

	for i := range existing {
		key := PermissionKey(existing[i].Permission)
		statsMap[key] = &existing[i]
	}

	for _, stat := range new {
		key := PermissionKey(stat.Permission)
		if existing, ok := statsMap[key]; ok {
			existing.Count += stat.Count
			if stat.LastSeen.After(existing.LastSeen) {
				existing.LastSeen = stat.LastSeen
			}
		} else {
			statCopy := stat
			statsMap[key] = &statCopy
		}
	}

	result := make([]types.PermissionStats, 0, len(statsMap))
	for _, stat := range statsMap {
		result = append(result, *stat)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}
```

**Step 2: Build and verify**

Run: `go build ./...`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/parser/agent_usage.go
git commit -m "feat: add parser to extract agent permission usage from session logs"
```

---

## Task 3: Integrate Agent Usage into Model

**Files:**
- Modify: `internal/types.go`
- Modify: `internal/model.go`

**Step 1: Add agentUsage field to Model**

In `internal/types.go`, add to Model struct after `skills`:

```go
	// Agent usage stats (from session logs)
	agentUsage []types.AgentUsageStats
```

**Step 2: Add to dataLoadedMsg**

In `internal/model.go`, update `dataLoadedMsg` struct:

```go
type dataLoadedMsg struct {
	permissions      []types.PermissionStats
	permissionGroups []types.PermissionGroup
	agents           []types.AgentPermissions
	skills           []types.SkillPermissions
	agentUsage       []types.AgentUsageStats // Add this
	userApproved     []string
	projectApproved  []string
	err              error
}
```

**Step 3: Load agent usage in LoadData**

In `internal/model.go`, in the `LoadData` function, add after loading skills:

```go
	// Load agent usage stats from session logs
	agentUsage, _ := parser.LoadAgentUsageStats()
```

And include in return:

```go
	return dataLoadedMsg{
		permissions:      permissions,
		permissionGroups: groups,
		agents:           agents,
		skills:           skills,
		agentUsage:       agentUsage,
		userApproved:     userApproved,
		projectApproved:  projectApproved,
		err:              nil,
	}
```

**Step 4: Handle in Update**

In `internal/update.go`, in the `dataLoadedMsg` case, add:

```go
	m.agentUsage = msg.agentUsage
```

**Step 5: Build and verify**

Run: `go build ./...`
Expected: Compiles without errors

**Step 6: Commit**

```bash
git add internal/types.go internal/model.go internal/update.go
git commit -m "feat: integrate agent usage stats into model loading"
```

---

## Task 4: Create Responsive Matrix Table Layout

**Files:**
- Modify: `internal/view_matrix.go`

**Step 1: Rewrite renderMatrixView for responsive columns**

Replace the `renderMatrixView` function in `internal/view_matrix.go`:

```go
// renderMatrixView renders the agent/skill permission matrix with responsive columns
func (m Model) renderMatrixView() string {
	_, contentHeight := m.calculateLayout()

	var lines []string

	// Header with counts
	declaredCount := len(m.agents)
	usageCount := len(m.agentUsage)
	header := fmt.Sprintf("Agents: %d declared | %d with usage data", declaredCount, usageCount)
	lines = append(lines, truncateString(header, m.width-4))
	lines = append(lines, strings.Repeat("─", m.width-4))

	// Column header
	colHeader := m.renderMatrixColumnHeader()
	lines = append(lines, colHeader)

	// Available lines for list
	viewportHeight := contentHeight - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Use agent usage stats if available, otherwise fall back to declared agents
	if len(m.agentUsage) > 0 {
		scrollOffset := m.matrixScroll
		endIdx := scrollOffset + viewportHeight
		if endIdx > len(m.agentUsage) {
			endIdx = len(m.agentUsage)
		}

		for i := scrollOffset; i < endIdx; i++ {
			isSelected := i == m.matrixCursor
			lines = append(lines, m.renderAgentUsageRow(m.agentUsage[i], isSelected))
		}

		if endIdx < len(m.agentUsage) {
			remaining := len(m.agentUsage) - endIdx
			lines = append(lines, fmt.Sprintf("    ... %d more (↓)", remaining))
		}
	} else if len(m.agents) > 0 {
		// Fallback to declared agents
		scrollOffset := m.matrixScroll
		endIdx := scrollOffset + viewportHeight
		if endIdx > len(m.agents) {
			endIdx = len(m.agents)
		}

		for i := scrollOffset; i < endIdx; i++ {
			isSelected := i == m.matrixCursor
			lines = append(lines, renderAgentRowWithCursor(m.agents[i], m.width, isSelected))
		}
	} else {
		lines = append(lines, "")
		lines = append(lines, "  No agents found")
	}

	// Pad to content height
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines[:contentHeight], "\n") + "\n"
}

// renderMatrixColumnHeader renders the column headers
func (m Model) renderMatrixColumnHeader() string {
	// Calculate responsive widths
	nameWidth, declWidth, callsWidth, lastWidth, statusWidth := m.calculateMatrixColumns()

	name := padRight("Agent", nameWidth)
	decl := padLeft("Decl", declWidth)
	calls := padLeft("Calls", callsWidth)
	last := padLeft("Last", lastWidth)
	status := padLeft("Status", statusWidth)

	header := fmt.Sprintf("  %s %s %s %s %s", name, decl, calls, last, status)
	return styles.ListHeader.Render(truncateString(header, m.width-2))
}

// calculateMatrixColumns returns responsive column widths based on terminal width
func (m Model) calculateMatrixColumns() (nameWidth, declWidth, callsWidth, lastWidth, statusWidth int) {
	// Fixed columns
	declWidth = 6
	callsWidth = 7
	lastWidth = 10
	statusWidth = 8

	// Cursor + spacing
	fixedTotal := 2 + declWidth + callsWidth + lastWidth + statusWidth + 5 // 5 for spacing

	// Name gets remaining space (minimum 20)
	nameWidth = m.width - fixedTotal
	if nameWidth < 20 {
		nameWidth = 20
	}
	if nameWidth > 50 {
		nameWidth = 50
	}

	return
}

// renderAgentUsageRow renders a single agent usage row with responsive columns
func (m Model) renderAgentUsageRow(agent types.AgentUsageStats, selected bool) string {
	nameWidth, declWidth, callsWidth, lastWidth, statusWidth := m.calculateMatrixColumns()

	// Agent name
	name := truncateString(agent.AgentType, nameWidth)
	name = padRight(name, nameWidth)

	// Declared perms (find matching declared agent)
	declCount := m.getDeclaredPermCount(agent.AgentType)
	decl := padLeft(fmt.Sprintf("%d", declCount), declWidth)

	// Actual calls
	calls := padLeft(fmt.Sprintf("%d", agent.TotalCalls), callsWidth)

	// Last seen
	last := padLeft(formatRelativeTime(agent.LastSeen), lastWidth)

	// Approval status (count how many of this agent's perms are approved)
	approvedCount := m.countApprovedPerms(agent.Permissions)
	totalPerms := len(agent.Permissions)
	var status string
	if approvedCount == totalPerms && totalPerms > 0 {
		status = styles.StatusApproved.Render(padLeft("✓ all", statusWidth))
	} else if approvedCount > 0 {
		status = padLeft(fmt.Sprintf("%d/%d", approvedCount, totalPerms), statusWidth)
	} else {
		status = styles.StatusPending.Render(padLeft("○", statusWidth))
	}

	// Build line
	cursor := "  "
	if selected {
		cursor = "> "
	}

	line := fmt.Sprintf("%s%s %s %s %s %s", cursor, name, decl, calls, last, status)

	if selected {
		return styles.ListItemSelected.Render(truncateString(line, m.width-2))
	}
	return truncateString(line, m.width-2)
}

// getDeclaredPermCount finds declared permission count for an agent type
func (m Model) getDeclaredPermCount(agentType string) int {
	for _, agent := range m.agents {
		fullName := agent.Name
		if agent.Plugin != "" {
			fullName = agent.Plugin + ":" + agent.Name
		}
		if fullName == agentType {
			return len(agent.Permissions)
		}
	}
	return 0
}

// countApprovedPerms counts how many permissions are approved
func (m Model) countApprovedPerms(perms []types.PermissionStats) int {
	count := 0
	for _, p := range perms {
		for _, approved := range m.userApproved {
			if approved == p.Permission.Raw {
				count++
				break
			}
		}
		for _, approved := range m.projectApproved {
			if approved == p.Permission.Raw {
				count++
				break
			}
		}
	}
	return count
}
```

**Step 2: Build and verify**

Run: `go build -o bin/perms ./cmd/perms/`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/view_matrix.go
git commit -m "feat: responsive column layout for Matrix view with agent usage data"
```

---

## Task 5: Add Agent Detail Modal State

**Files:**
- Modify: `internal/types.go`

**Step 1: Add modal state fields**

In `internal/types.go`, update the Matrix view state section:

```go
	// Matrix view state
	matrixCursor     int  // Cursor position in agent list
	matrixScroll     int  // Scroll offset for viewport
	showAgentModal   bool // Show agent detail modal
	selectedAgentIdx int  // Index of agent for detail modal

	// Agent detail modal state
	agentModalCursor    int    // Cursor in permission list
	agentModalSelected  []bool // Which permissions are selected (toggled)
	agentModalMode      int    // 0=permission select, 1=scope select, 2=project select
	agentModalScope     int    // 0=user, 1=project
	agentModalProjCursor int   // Cursor in project list
```

**Step 2: Build and verify**

Run: `go build ./...`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/types.go
git commit -m "feat: add agent detail modal state fields"
```

---

## Task 6: Implement Agent Detail Modal Rendering

**Files:**
- Modify: `internal/view_matrix.go`

**Step 1: Rewrite renderAgentDetailModal**

Replace the existing `renderAgentDetailModal` function:

```go
// Agent modal modes
const (
	AgentModalModePermissions = iota
	AgentModalModeScope
	AgentModalModeProject
)

// renderAgentDetailModal renders the agent detail modal with multi-select
func (m Model) renderAgentDetailModal() string {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return ""
	}

	agent := m.agentUsage[m.selectedAgentIdx]

	modalWidth := m.width * 80 / 100
	if modalWidth > 70 {
		modalWidth = 70
	}
	if modalWidth < 50 {
		modalWidth = 50
	}

	var content strings.Builder

	// Header
	content.WriteString(styles.ModalTitle.Render(agent.AgentType))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("  %d total calls across %d sessions\n", agent.TotalCalls, agent.Sessions))
	content.WriteString("\n")

	switch m.agentModalMode {
	case AgentModalModePermissions:
		content.WriteString(m.renderPermissionSelectMode(agent))
	case AgentModalModeScope:
		content.WriteString(m.renderScopeSelectMode())
	case AgentModalModeProject:
		content.WriteString(m.renderProjectSelectMode(agent))
	}

	return styles.Modal.Width(modalWidth).Render(content.String())
}

// renderPermissionSelectMode renders the permission multi-select list
func (m Model) renderPermissionSelectMode(agent types.AgentUsageStats) string {
	var content strings.Builder

	content.WriteString("  Permissions requested by this agent:\n\n")

	selectedCount := 0
	for i, perm := range agent.Permissions {
		isSelected := i < len(m.agentModalSelected) && m.agentModalSelected[i]
		isCursor := i == m.agentModalCursor

		// Checkbox
		checkbox := "[ ]"
		if isSelected {
			checkbox = "[x]"
			selectedCount++
		}

		// Cursor indicator
		cursor := "  "
		if isCursor {
			cursor = "> "
		}

		// Permission name and count
		permName := truncateString(perm.Permission.Raw, 30)
		calls := fmt.Sprintf("%d calls", perm.Count)

		// Approval status
		status := m.getPermissionApprovalStatus(perm.Permission.Raw)

		line := fmt.Sprintf("%s%s %-30s %10s  %s", cursor, checkbox, permName, calls, status)

		if isCursor {
			content.WriteString(styles.ListItemSelected.Render(line))
		} else {
			content.WriteString(line)
		}
		content.WriteString("\n")
	}

	content.WriteString(fmt.Sprintf("\n  %d selected\n\n", selectedCount))
	content.WriteString(fmt.Sprintf("  %s Toggle  %s Navigate  %s Apply  %s Close",
		styles.HelpKey.Render("Space"),
		styles.HelpKey.Render("j/k"),
		styles.HelpKey.Render("A"),
		styles.HelpKey.Render("Esc")))

	return content.String()
}

// renderScopeSelectMode renders the user/project scope selection
func (m Model) renderScopeSelectMode() string {
	var content strings.Builder

	selectedCount := 0
	for _, sel := range m.agentModalSelected {
		if sel {
			selectedCount++
		}
	}

	content.WriteString(fmt.Sprintf("  Apply %d permissions to:\n\n", selectedCount))

	userCursor := "  "
	projCursor := "  "
	if m.agentModalScope == 0 {
		userCursor = "> "
	} else {
		projCursor = "> "
	}

	userLine := fmt.Sprintf("%sUser level (~/.claude/settings.local.json)", userCursor)
	projLine := fmt.Sprintf("%sProject level", projCursor)

	if m.agentModalScope == 0 {
		content.WriteString(styles.ListItemSelected.Render(userLine))
	} else {
		content.WriteString(userLine)
	}
	content.WriteString("\n")

	if m.agentModalScope == 1 {
		content.WriteString(styles.ListItemSelected.Render(projLine))
	} else {
		content.WriteString(projLine)
	}
	content.WriteString("\n")

	content.WriteString(fmt.Sprintf("\n  %s Select  %s Navigate  %s Back",
		styles.HelpKey.Render("Enter"),
		styles.HelpKey.Render("j/k"),
		styles.HelpKey.Render("Esc")))

	return content.String()
}

// renderProjectSelectMode renders the project selection list
func (m Model) renderProjectSelectMode(agent types.AgentUsageStats) string {
	var content strings.Builder

	content.WriteString("  Select project:\n\n")

	for i, proj := range agent.Projects {
		cursor := "  "
		if i == m.agentModalProjCursor {
			cursor = "> "
		}

		line := fmt.Sprintf("%s%s", cursor, proj)

		if i == m.agentModalProjCursor {
			content.WriteString(styles.ListItemSelected.Render(line))
		} else {
			content.WriteString(line)
		}
		content.WriteString("\n")
	}

	content.WriteString(fmt.Sprintf("\n  %s Select  %s Navigate  %s Back",
		styles.HelpKey.Render("Enter"),
		styles.HelpKey.Render("j/k"),
		styles.HelpKey.Render("Esc")))

	return content.String()
}

// getPermissionApprovalStatus returns approval status string for a permission
func (m Model) getPermissionApprovalStatus(permRaw string) string {
	for _, approved := range m.userApproved {
		if approved == permRaw {
			return styles.StatusApproved.Render("✓ user")
		}
	}
	for _, approved := range m.projectApproved {
		if approved == permRaw {
			return styles.StatusApproved.Render("✓ proj")
		}
	}
	return styles.StatusPending.Render("○")
}
```

**Step 2: Build and verify**

Run: `go build -o bin/perms ./cmd/perms/`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/view_matrix.go
git commit -m "feat: implement agent detail modal with multi-select permissions"
```

---

## Task 7: Implement Agent Modal Keyboard Handling

**Files:**
- Modify: `internal/update_keyboard.go`

**Step 1: Replace agent modal key handling**

In `internal/update_keyboard.go`, replace the agent modal handling block:

```go
	// Handle agent detail modal
	if m.showAgentModal {
		return m.handleAgentModalKeys(msg)
	}
```

**Step 2: Add handleAgentModalKeys function**

Add this function to `internal/update_keyboard.go`:

```go
// handleAgentModalKeys processes keys while agent detail modal is open
func (m Model) handleAgentModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.agentModalMode {
	case AgentModalModePermissions:
		return m.handleAgentPermissionKeys(msg)
	case AgentModalModeScope:
		return m.handleAgentScopeKeys(msg)
	case AgentModalModeProject:
		return m.handleAgentProjectKeys(msg)
	}
	return m, nil
}

func (m Model) handleAgentPermissionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]
	maxIdx := len(agent.Permissions) - 1

	switch msg.String() {
	case "esc", "q":
		m.showAgentModal = false
		m.resetAgentModalState()
		return m, nil

	case "j", "down":
		if m.agentModalCursor < maxIdx {
			m.agentModalCursor++
		}
		return m, nil

	case "k", "up":
		if m.agentModalCursor > 0 {
			m.agentModalCursor--
		}
		return m, nil

	case " ": // Spacebar to toggle
		if m.agentModalCursor <= maxIdx {
			// Ensure slice is big enough
			for len(m.agentModalSelected) <= m.agentModalCursor {
				m.agentModalSelected = append(m.agentModalSelected, false)
			}
			m.agentModalSelected[m.agentModalCursor] = !m.agentModalSelected[m.agentModalCursor]
		}
		return m, nil

	case "a", "A": // Apply selected
		// Check if any selected
		hasSelected := false
		for _, sel := range m.agentModalSelected {
			if sel {
				hasSelected = true
				break
			}
		}
		if hasSelected {
			m.agentModalMode = AgentModalModeScope
			m.agentModalScope = 0
		}
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleAgentScopeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.agentModalMode = AgentModalModePermissions
		return m, nil

	case "j", "down":
		if m.agentModalScope < 1 {
			m.agentModalScope++
		}
		return m, nil

	case "k", "up":
		if m.agentModalScope > 0 {
			m.agentModalScope--
		}
		return m, nil

	case "enter", " ":
		if m.agentModalScope == 0 {
			// Apply to user level
			return m.applySelectedToUser()
		} else {
			// Go to project selection
			m.agentModalMode = AgentModalModeProject
			m.agentModalProjCursor = 0
		}
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleAgentProjectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]
	maxIdx := len(agent.Projects) - 1

	switch msg.String() {
	case "esc":
		m.agentModalMode = AgentModalModeScope
		return m, nil

	case "j", "down":
		if m.agentModalProjCursor < maxIdx {
			m.agentModalProjCursor++
		}
		return m, nil

	case "k", "up":
		if m.agentModalProjCursor > 0 {
			m.agentModalProjCursor--
		}
		return m, nil

	case "enter", " ":
		return m.applySelectedToProject()

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

// applySelectedToUser writes selected permissions to user settings
func (m Model) applySelectedToUser() (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]

	for i, perm := range agent.Permissions {
		if i < len(m.agentModalSelected) && m.agentModalSelected[i] {
			if err := parser.WritePermissionToUserSettings(perm.Permission.Raw); err != nil {
				m.err = err
				return m, nil
			}
			m.userApproved = append(m.userApproved, perm.Permission.Raw)
		}
	}

	m.showAgentModal = false
	m.resetAgentModalState()
	return m, nil
}

// applySelectedToProject writes selected permissions to project settings
func (m Model) applySelectedToProject() (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]

	if m.agentModalProjCursor >= len(agent.Projects) {
		return m, nil
	}
	projectPath := agent.Projects[m.agentModalProjCursor]

	for i, perm := range agent.Permissions {
		if i < len(m.agentModalSelected) && m.agentModalSelected[i] {
			if err := parser.WritePermissionToProjectSettings(projectPath, perm.Permission.Raw); err != nil {
				m.err = err
				return m, nil
			}
			m.projectApproved = append(m.projectApproved, perm.Permission.Raw)
		}
	}

	m.showAgentModal = false
	m.resetAgentModalState()
	return m, nil
}

// resetAgentModalState resets all agent modal state
func (m *Model) resetAgentModalState() {
	m.agentModalCursor = 0
	m.agentModalSelected = nil
	m.agentModalMode = 0
	m.agentModalScope = 0
	m.agentModalProjCursor = 0
}
```

**Step 3: Add AgentModalMode constants at package level**

At the top of `internal/update_keyboard.go` or in `internal/types.go`, add:

```go
// Agent modal modes
const (
	AgentModalModePermissions = iota
	AgentModalModeScope
	AgentModalModeProject
)
```

**Step 4: Build and verify**

Run: `go build -o bin/perms ./cmd/perms/`
Expected: Compiles without errors

**Step 5: Commit**

```bash
git add internal/update_keyboard.go
git commit -m "feat: implement agent modal keyboard navigation and apply actions"
```

---

## Task 8: Update Status Bar for Matrix View

**Files:**
- Modify: `internal/view.go`

**Step 1: Update status bar text for Matrix view**

In `internal/view.go`, find the `renderStatusBar` function and update the Matrix case:

```go
		case ViewMatrix:
			if len(m.agentUsage) > 0 {
				left = fmt.Sprintf("%d/%d agents", m.matrixCursor+1, len(m.agentUsage))
			} else if len(m.agents) > 0 {
				left = fmt.Sprintf("%d/%d agents", m.matrixCursor+1, len(m.agents))
			} else {
				left = "No agents found"
			}
```

And update the right side help text:

```go
	right = "j/k: nav  Enter: details  Tab: view  /: filter  q: quit"
```

**Step 2: Build and test**

Run: `go build -o bin/perms ./cmd/perms/ && ./bin/perms`
Expected: Status bar shows "Enter: details" and correct agent count

**Step 3: Commit**

```bash
git add internal/view.go
git commit -m "fix: update status bar to show 'Enter: details' for Matrix view"
```

---

## Task 9: Fix Matrix Navigation for Agent Usage

**Files:**
- Modify: `internal/model.go`

**Step 1: Update navigateMatrixDown and navigateMatrixUp**

Ensure navigation uses agentUsage length when available:

```go
func (m *Model) navigateMatrixDown() {
	maxIdx := len(m.agentUsage) - 1
	if maxIdx < 0 {
		maxIdx = len(m.agents) - 1
	}
	if maxIdx < 0 {
		return
	}

	if m.matrixCursor < maxIdx {
		m.matrixCursor++

		// Scroll if needed
		_, contentHeight := m.calculateLayout()
		viewportHeight := contentHeight - 5
		if m.matrixCursor >= m.matrixScroll+viewportHeight {
			m.matrixScroll++
		}
	}
}

func (m *Model) navigateMatrixUp() {
	if m.matrixCursor > 0 {
		m.matrixCursor--

		// Scroll if needed
		if m.matrixCursor < m.matrixScroll {
			m.matrixScroll = m.matrixCursor
		}
	}
}
```

**Step 2: Build and test**

Run: `go build -o bin/perms ./cmd/perms/`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add internal/model.go
git commit -m "fix: matrix navigation uses agentUsage length when available"
```

---

## Task 10: Initialize Agent Modal Selected Slice on Enter

**Files:**
- Modify: `internal/update_keyboard.go`

**Step 1: Initialize selection state when opening modal**

In the Enter key handler for Matrix view, initialize the selection slice:

```go
		case ViewMatrix:
			if len(m.agentUsage) > 0 && m.matrixCursor < len(m.agentUsage) {
				m.selectedAgentIdx = m.matrixCursor
				m.showAgentModal = true
				// Initialize selection state
				m.agentModalSelected = make([]bool, len(m.agentUsage[m.matrixCursor].Permissions))
				m.agentModalCursor = 0
				m.agentModalMode = AgentModalModePermissions
			}
```

**Step 2: Build and test full flow**

Run: `go build -o bin/perms ./cmd/perms/ && ./bin/perms`

Test:
1. Tab to Matrix view
2. Navigate with j/k
3. Press Enter on an agent
4. Verify modal shows permissions with [ ] checkboxes
5. Press Space to toggle [x]
6. Press A to go to scope selection
7. Press Enter to apply to user level
8. Verify modal closes and permissions are approved

**Step 3: Commit**

```bash
git add internal/update_keyboard.go
git commit -m "feat: initialize agent modal selection state on Enter"
```

---

## Summary

After completing all tasks:

1. ✅ New parser correlates Task calls to subagent sessions via slug
2. ✅ AgentUsageStats tracks actual permission usage per agent type
3. ✅ Matrix view shows responsive table with columns: Agent | Declared | Calls | Last | Status
4. ✅ Enter opens detail modal showing agent's actual permissions
5. ✅ Spacebar toggles permission selection
6. ✅ A opens scope selection (User vs Project)
7. ✅ Project selection if needed
8. ✅ Selected permissions written to appropriate settings file
