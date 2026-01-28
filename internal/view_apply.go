package internal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderApplyModal renders the apply permission modal
func (m Model) renderApplyModal() string {
	perm := m.selectedPermission()
	if perm == nil {
		return ""
	}

	// Use 80% of terminal width, max 60, min 40
	modalWidth := m.width * 80 / 100
	if modalWidth > 60 {
		modalWidth = 60
	}
	if modalWidth < 40 {
		modalWidth = 40
	}

	// Build modal content
	var content strings.Builder

	// Title
	title := styles.ModalTitle.Render("Apply Permission")
	content.WriteString(title)
	content.WriteString("\n\n")

	// Permission info
	content.WriteString(fmt.Sprintf("  Permission: %s\n", styles.HelpKey.Render(perm.Permission.Raw)))
	content.WriteString(fmt.Sprintf("  Requested: %d times across %d project(s)\n",
		perm.Count, len(perm.Projects)))
	content.WriteString("\n")

	// User-level box
	userBox := renderCommandBox(
		"User-level (all projects)",
		"Add to ~/.claude/settings.local.json:",
		perm.Permission.Raw,
		modalWidth-4,
	)
	content.WriteString(userBox)
	content.WriteString("\n")

	// Project-level box
	projectBox := renderCommandBox(
		"Project-level (current project)",
		"Add to .claude/settings.local.json:",
		perm.Permission.Raw,
		modalWidth-4,
	)
	content.WriteString(projectBox)
	content.WriteString("\n\n")

	// Instructions
	instructions := fmt.Sprintf("%s Copy user cmd  %s Copy project cmd  %s Back",
		styles.HelpKey.Render("[u]"),
		styles.HelpKey.Render("[p]"),
		styles.HelpKey.Render("[Esc]"),
	)
	content.WriteString(instructions)

	// Wrap in modal border
	modal := styles.Modal.
		Width(modalWidth).
		Render(content.String())

	return modal
}

// renderCommandBox renders a command box with title and content
func renderCommandBox(title, description, permission string, width int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorSecondary).
		Padding(0, 1).
		Width(width)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary)

	var content strings.Builder
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n")
	content.WriteString(description)
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf(`"permissions": { "allow": ["%s"] }`, permission))

	return boxStyle.Render(content.String())
}
