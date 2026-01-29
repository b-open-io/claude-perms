package internal

import (
	"fmt"
	"strings"

	"github.com/b-open-io/claude-perms/internal/types"
)

// calculateMatrixColumns returns responsive column widths based on terminal width
// Returns: nameWidth, declWidth, callsWidth, lastWidth, statusWidth
func (m Model) calculateMatrixColumns() (nameWidth, declWidth, callsWidth, lastWidth, statusWidth int) {
	// Fixed column widths
	declWidth = 6   // "Decl" column (right-aligned number)
	callsWidth = 7  // "Calls" column (right-aligned number)
	lastWidth = 10  // "Last" column (relative time like "2h ago")
	statusWidth = 8 // "Status" column ("all", "3/5", or "-")

	// Spacing between columns: 1 space each side = 5 total gaps between 5 columns
	const cursorWidth = 2   // "> " or "  "
	const columnSpacing = 5 // 1 space between each of the 5 columns

	// Calculate remaining space for name column
	fixedWidth := cursorWidth + declWidth + callsWidth + lastWidth + statusWidth + columnSpacing
	nameWidth = m.width - fixedWidth - 4 // -4 for padding

	// Clamp name width between min and max
	if nameWidth < 20 {
		nameWidth = 20
	}
	if nameWidth > 50 {
		nameWidth = 50
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
		// Check if permission is in user or project approved lists
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

	if selected {
		return styles.ListItemSelected.Render(truncateString(row, m.width-2))
	}
	return truncateString(row, m.width-2)
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
	header := truncateString(fmt.Sprintf("Agents: %d | Skills: %d", agentCount, skillCount), m.width-4)
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("-", m.width-4))

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

// renderAgentRow renders a single agent row (must be single line, no wrapping)
func renderAgentRow(agent types.AgentPermissions, width int) string {
	return renderAgentRowWithCursor(agent, width, false)
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

	if selected {
		return styles.ListItemSelected.Render(truncateString(line, width-2))
	}
	return truncateString(line, width-2)
}

// renderSkillRow renders a single skill row (must be single line, no wrapping)
func renderSkillRow(skill types.SkillPermissions, width int) string {
	var name string
	if skill.Plugin != "" {
		name = fmt.Sprintf("%s:%s", skill.Plugin, skill.Name)
	} else {
		name = skill.Name
	}

	permCount := fmt.Sprintf("(%d perms)", len(skill.Permissions))

	// Calculate available width
	maxNameWidth := width - 4 - len(permCount) - 2
	if maxNameWidth < 10 {
		maxNameWidth = 10
	}
	name = truncateString(name, maxNameWidth)

	line := fmt.Sprintf("    %s  %s", padRight(name, maxNameWidth), permCount)
	return truncateString(line, width-2)
}

// renderAgentDetailModal renders the agent detail modal
func (m Model) renderAgentDetailModal() string {
	if m.selectedAgentIdx >= len(m.agents) {
		return ""
	}

	agent := m.agents[m.selectedAgentIdx]

	modalWidth := m.width * 80 / 100
	if modalWidth > 70 {
		modalWidth = 70
	}
	if modalWidth < 40 {
		modalWidth = 40
	}

	var content strings.Builder

	// Header
	title := agent.Name
	if agent.Plugin != "" {
		title = agent.Plugin + ":" + agent.Name
	}
	content.WriteString(styles.ModalTitle.Render(title))
	content.WriteString("\n\n")

	// Metadata
	if agent.Plugin != "" {
		content.WriteString(fmt.Sprintf("  Plugin: %s\n", agent.Plugin))
	}
	if agent.Version != "" {
		content.WriteString(fmt.Sprintf("  Version: %s\n", agent.Version))
	}
	content.WriteString("\n")

	// Permissions list
	content.WriteString("  Declared Permissions:\n")
	for _, perm := range agent.Permissions {
		content.WriteString(fmt.Sprintf("    - %s\n", perm.Raw))
	}

	content.WriteString(fmt.Sprintf("\n%s close", styles.HelpKey.Render("Esc")))

	return styles.Modal.Width(modalWidth).Render(content.String())
}

// renderWithAgentModal overlays the agent detail modal
func (m Model) renderWithAgentModal(background string) string {
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
