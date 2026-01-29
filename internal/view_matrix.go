package internal

import (
	"fmt"
	"strings"

	"github.com/b-open-io/claude-perms/internal/parser"
	"github.com/b-open-io/claude-perms/internal/types"
)

// Agent modal modes
const (
	AgentModalModePermissions = iota
	AgentModalModeScope
	AgentModalModeProject
)

// calculateMatrixColumns returns responsive column widths based on terminal width
// Uses weight-based sizing so columns scale proportionally to fill the terminal.
// Returns: nameWidth, declWidth, callsWidth, lastWidth, statusWidth
func (m Model) calculateMatrixColumns() (nameWidth, declWidth, callsWidth, lastWidth, statusWidth int) {
	const cursorWidth = 2   // "> " or "  "
	const columnGaps = 5    // 1 space between each of the 5 columns
	const contentPad = 4    // Content area padding

	// Base column widths (minimums)
	declWidth = 6   // "Decl"
	callsWidth = 7  // "Calls"
	lastWidth = 10  // "Last" (e.g. "just now", "2w ago")
	statusWidth = 8 // "Status" (e.g. "all", "3/5")

	fixedWidth := cursorWidth + declWidth + callsWidth + lastWidth + statusWidth + columnGaps + contentPad
	nameWidth = m.width - fixedWidth

	// On wide terminals (120+), distribute extra space to data columns too
	extra := nameWidth - 45 // space beyond a comfortable agent name width
	if extra > 0 {
		// Give some extra breathing room to data columns
		bonus := extra / 4
		if bonus > 4 {
			bonus = 4
		}
		declWidth += bonus
		callsWidth += bonus
		lastWidth += bonus
		statusWidth += bonus
		// Recalculate name from the new fixed total
		fixedWidth = cursorWidth + declWidth + callsWidth + lastWidth + statusWidth + columnGaps + contentPad
		nameWidth = m.width - fixedWidth
	}

	// Clamp name width to a sensible minimum
	if nameWidth < 20 {
		nameWidth = 20
	}

	return nameWidth, declWidth, callsWidth, lastWidth, statusWidth
}

// renderMatrixColumnHeader renders the column headers
func (m Model) renderMatrixColumnHeader() string {
	nameWidth, declWidth, callsWidth, lastWidth, statusWidth := m.calculateMatrixColumns()

	// Build header with column names
	agent := padRight("Agent", nameWidth)
	decl := padLeft("Decl", declWidth)
	calls := padLeft("Calls", callsWidth)
	last := padLeft("Last", lastWidth)
	status := padLeft("Status", statusWidth)

	header := fmt.Sprintf("  %s %s %s %s %s", agent, decl, calls, last, status)
	header = padRight(header, m.width-4)
	return styles.ListHeader.Render(header)
}

// getDeclaredPermCount finds declared permission count for an agent type
func (m Model) getDeclaredPermCount(agentType string) int {
	for _, agent := range m.agents {
		// Match by name (with or without plugin prefix)
		fullName := agent.Name
		if agent.Plugin != "" {
			fullName = agent.Plugin + ":" + agent.Name
		}
		if fullName == agentType || agent.Name == agentType {
			return len(agent.Permissions)
		}
	}
	return 0
}

// countApprovedPerms counts how many permissions are approved
func (m Model) countApprovedPerms(perms []types.PermissionStats) int {
	count := 0
	for _, p := range perms {
		approved := false
		// Check if permission is in user approved list
		for _, a := range m.userApproved {
			if a == p.Permission.Raw {
				approved = true
				break
			}
		}
		// Only check project approved if not already found
		if !approved {
			for _, a := range m.projectApproved {
				if a == p.Permission.Raw {
					approved = true
					break
				}
			}
		}
		if approved {
			count++
		}
	}
	return count
}

// renderAgentUsageRow renders a single agent usage row with responsive columns
func (m Model) renderAgentUsageRow(agent types.AgentUsageStats, selected bool) string {
	nameWidth, declWidth, callsWidth, lastWidth, statusWidth := m.calculateMatrixColumns()

	// Agent name (truncated if needed)
	name := truncateString(agent.AgentType, nameWidth)
	name = padRight(name, nameWidth)

	// Declared permission count
	declCount := m.getDeclaredPermCount(agent.AgentType)
	decl := padLeft(fmt.Sprintf("%d", declCount), declWidth)

	// Actual call count
	calls := padLeft(fmt.Sprintf("%d", agent.TotalCalls), callsWidth)

	// Last seen time
	last := padLeft(formatRelativeTime(agent.LastSeen), lastWidth)

	// Status: show approval ratio or indicator
	var statusText string
	permCount := len(agent.Permissions)
	if permCount == 0 {
		statusText = "-"
	} else {
		approvedCount := m.countApprovedPerms(agent.Permissions)
		if approvedCount == permCount {
			statusText = "all"
		} else {
			statusText = fmt.Sprintf("%d/%d", approvedCount, permCount)
		}
	}
	status := padLeft(statusText, statusWidth)

	// Build the row
	cursor := "  "
	if selected {
		cursor = "> "
	}

	row := fmt.Sprintf("%s%s %s %s %s %s", cursor, name, decl, calls, last, status)

	// Pad row to fill terminal width for full-width highlight
	maxWidth := m.width - 2
	row = truncateString(row, maxWidth)
	row = padRight(row, maxWidth)

	if selected {
		return styles.ListItemSelected.Render(row)
	}
	return row
}

// renderMatrixView renders the agent/skill permission matrix with responsive columns
func (m Model) renderMatrixView() string {
	_, contentHeight := m.calculateLayout()

	var lines []string

	// Section header (ensure single line)
	agentCount := len(m.agentUsage)
	if agentCount == 0 {
		agentCount = len(m.agents)
	}
	skillCount := len(m.skills)
	headerText := fmt.Sprintf("Agents: %d | Skills: %d", agentCount, skillCount)
	lines = append(lines, padRight(truncateString(headerText, m.width-4), m.width-4))
	lines = append(lines, strings.Repeat("─", m.width-4))

	// Column headers
	lines = append(lines, m.renderMatrixColumnHeader())

	// Available lines for agent list (after header + separator + column header)
	viewportHeight := contentHeight - 3
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Use stored scroll offset (maintained by navigation methods)
	scrollOffset := m.matrixScroll

	// Prefer agent usage data if available, otherwise fall back to declared agents
	if len(m.agentUsage) > 0 {
		// Show scroll-up indicator if not at top
		if scrollOffset > 0 {
			lines = append(lines, fmt.Sprintf("  (^ %d more)", scrollOffset))
			viewportHeight--
		}

		// Render visible window of agents
		endIdx := scrollOffset + viewportHeight
		if endIdx > len(m.agentUsage) {
			endIdx = len(m.agentUsage)
		}

		for i := scrollOffset; i < endIdx; i++ {
			isSelected := i == m.matrixCursor
			lines = append(lines, m.renderAgentUsageRow(m.agentUsage[i], isSelected))
		}

		// Show scroll-down indicator if there's more content
		if endIdx < len(m.agentUsage) {
			remaining := len(m.agentUsage) - endIdx
			lines = append(lines, fmt.Sprintf("  ... %d more (v)", remaining))
		}
	} else if len(m.agents) > 0 {
		// Fallback to declared agents
		if scrollOffset > 0 {
			lines = append(lines, fmt.Sprintf("  (^ %d more)", scrollOffset))
			viewportHeight--
		}

		endIdx := scrollOffset + viewportHeight
		if endIdx > len(m.agents) {
			endIdx = len(m.agents)
		}

		for i := scrollOffset; i < endIdx; i++ {
			isSelected := i == m.matrixCursor
			lines = append(lines, renderAgentRowWithCursor(m.agents[i], m.width, isSelected))
		}

		if endIdx < len(m.agents) {
			remaining := len(m.agents) - endIdx
			lines = append(lines, fmt.Sprintf("  ... %d more (v)", remaining))
		}
	} else {
		// Empty state
		lines = append(lines, "")
		lines = append(lines, "  No agents or skills with declared permissions")
	}

	// Pad to exact content height
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines[:contentHeight], "\n") + "\n"
}

// renderAgentRowWithCursor renders an agent row with optional cursor (fallback for declared agents)
func renderAgentRowWithCursor(agent types.AgentPermissions, width int, selected bool) string {
	var name string
	if agent.Plugin != "" {
		name = fmt.Sprintf("%s:%s", agent.Plugin, agent.Name)
	} else {
		name = agent.Name
	}

	permCount := fmt.Sprintf("(%d)", len(agent.Permissions))

	// Fixed column widths for clean alignment
	const nameCol = 45 // Agent name column width
	const permCol = 6  // Permission count column width

	// Truncate name if needed
	displayName := truncateString(name, nameCol)

	// Build line with cursor indicator
	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Format: cursor + name (left-aligned, fixed width) + perm count (right-aligned)
	line := fmt.Sprintf("%s%s %s", cursor, padRight(displayName, nameCol), padLeft(permCount, permCol))

	maxWidth := width - 2
	line = truncateString(line, maxWidth)
	line = padRight(line, maxWidth)

	if selected {
		return styles.ListItemSelected.Render(line)
	}
	return line
}

// renderAgentDetailModal renders the agent detail modal with multi-select
func (m Model) renderAgentDetailModal() string {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return ""
	}

	agent := m.agentUsage[m.selectedAgentIdx]

	modalWidth := m.width * 85 / 100
	if modalWidth > 80 {
		modalWidth = 80
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

		checkbox := "[ ]"
		if isSelected {
			checkbox = "[x]"
			selectedCount++
		}

		cursor := "  "
		if isCursor {
			cursor = "> "
		}

		permName := truncateString(perm.Permission.Raw, 30)
		calls := fmt.Sprintf("%d calls", perm.Count)
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
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return ""
	}
	agent := m.agentUsage[m.selectedAgentIdx]

	var content strings.Builder

	// Collect selected permission strings
	var selectedPerms []string
	for i, perm := range agent.Permissions {
		if i < len(m.agentModalSelected) && m.agentModalSelected[i] {
			selectedPerms = append(selectedPerms, perm.Permission.Raw)
		}
	}

	content.WriteString(fmt.Sprintf("  Apply %d permissions to:\n\n", len(selectedPerms)))

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

	// Diff preview for current scope selection
	if m.agentModalScope == 0 && len(selectedPerms) > 0 {
		content.WriteString("\n")
		filePath, diffLines, allExist := parser.PreviewUserDiff(selectedPerms)
		content.WriteString(renderDiffPreview(filePath, diffLines, allExist, 74))
	}

	content.WriteString(fmt.Sprintf("\n  %s Select  %s Navigate  %s Back",
		styles.HelpKey.Render("Enter"),
		styles.HelpKey.Render("j/k"),
		styles.HelpKey.Render("Esc")))

	return content.String()
}

// renderProjectSelectMode renders the project selection list
func (m Model) renderProjectSelectMode(agent types.AgentUsageStats) string {
	var content strings.Builder

	// Collect selected permission strings
	var selectedPerms []string
	for i, perm := range agent.Permissions {
		if i < len(m.agentModalSelected) && m.agentModalSelected[i] {
			selectedPerms = append(selectedPerms, perm.Permission.Raw)
		}
	}

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

	// Diff preview for the selected project
	if m.agentModalProjCursor < len(agent.Projects) && len(selectedPerms) > 0 {
		content.WriteString("\n")
		projectPath := agent.Projects[m.agentModalProjCursor]
		filePath, diffLines, allExist := parser.PreviewProjectDiff(projectPath, selectedPerms)
		content.WriteString(renderDiffPreview(filePath, diffLines, allExist, 74))
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

// renderWithAgentModal overlays the agent detail modal
func (m Model) renderWithAgentModal(_ string) string {
	modal := m.renderAgentDetailModal()

	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)

	topPadding := (m.height - modalHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	var result strings.Builder
	for i := 0; i < topPadding; i++ {
		result.WriteString("\n")
	}
	result.WriteString(modal)

	return result.String()
}
