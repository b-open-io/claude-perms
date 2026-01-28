package internal

import (
	"fmt"
	"strings"

	"github.com/b-open-io/claude-perms/internal/types"
)

// renderMatrixView renders the agent/skill permission matrix
func (m Model) renderMatrixView() string {
	_, contentHeight := m.calculateLayout()

	var lines []string

	// Section: Agents
	lines = append(lines, styles.ListHeader.Render("Agents with declared permissions:"))
	lines = append(lines, "")

	if len(m.agents) == 0 {
		lines = append(lines, styles.ListItem.Render("  No agents with declared permissions found"))
	} else {
		for _, agent := range m.agents {
			line := renderAgentRow(agent, m.width)
			lines = append(lines, line)

			// Show permissions (indented)
			for _, perm := range agent.Permissions {
				permLine := styles.ListItem.Render(fmt.Sprintf("    - %s", perm.Raw))
				lines = append(lines, permLine)
			}
			lines = append(lines, "")
		}
	}

	lines = append(lines, "")
	lines = append(lines, styles.ListHeader.Render("Skills with declared permissions:"))
	lines = append(lines, "")

	if len(m.skills) == 0 {
		lines = append(lines, styles.ListItem.Render("  No skills with declared permissions found"))
	} else {
		for _, skill := range m.skills {
			line := renderSkillRow(skill, m.width)
			lines = append(lines, line)

			// Show permissions (indented)
			for _, perm := range skill.Permissions {
				permLine := styles.ListItem.Render(fmt.Sprintf("    - %s", perm.Raw))
				lines = append(lines, permLine)
			}
			lines = append(lines, "")
		}
	}

	// Pad remaining lines
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines[:contentHeight], "\n") + "\n"
}

// renderAgentRow renders a single agent row
func renderAgentRow(agent types.AgentPermissions, width int) string {
	var name string
	if agent.Plugin != "" {
		name = fmt.Sprintf("%s:%s", agent.Plugin, agent.Name)
	} else {
		name = agent.Name
	}

	permCount := fmt.Sprintf("(%d permissions)", len(agent.Permissions))

	maxNameWidth := width - len(permCount) - 6
	name = truncateString(name, maxNameWidth)

	return styles.ListItem.Render(fmt.Sprintf("  %s %s", name, styles.ColTime.Render(permCount)))
}

// renderSkillRow renders a single skill row
func renderSkillRow(skill types.SkillPermissions, width int) string {
	var name string
	if skill.Plugin != "" {
		name = fmt.Sprintf("%s:%s", skill.Plugin, skill.Name)
	} else {
		name = skill.Name
	}

	permCount := fmt.Sprintf("(%d permissions)", len(skill.Permissions))

	maxNameWidth := width - len(permCount) - 6
	name = truncateString(name, maxNameWidth)

	return styles.ListItem.Render(fmt.Sprintf("  %s %s", name, styles.ColTime.Render(permCount)))
}
