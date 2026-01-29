package internal

import (
	"fmt"
	"strings"

	"github.com/b-open-io/claude-perms/internal/parser"
	"github.com/b-open-io/claude-perms/internal/types"
)

// renderApplyModal renders the apply permission modal
func (m Model) renderApplyModal() string {
	perm := m.selectedPermission()
	if perm == nil {
		return ""
	}

	modalWidth := m.width * 85 / 100
	if modalWidth > 80 {
		modalWidth = 80
	}
	if modalWidth < 50 {
		modalWidth = 50
	}

	var content strings.Builder

	title := styles.ModalTitle.Render("Apply Permission")
	content.WriteString(title)
	content.WriteString("\n\n")

	content.WriteString(fmt.Sprintf("  %s\n", styles.HelpKey.Render(perm.Permission.Raw)))
	content.WriteString(fmt.Sprintf("  %d uses across %d project(s)\n\n",
		perm.Count, len(perm.Projects)))

	switch m.applyModalMode {
	case ApplyModeOptionSelect:
		content.WriteString(m.renderOptionSelect())
	case ApplyModeProjectSelect:
		content.WriteString(m.renderProjectSelect(perm))
	}

	return styles.Modal.Width(modalWidth).Render(content.String())
}

func (m Model) renderOptionSelect() string {
	perm := m.selectedPermission()

	var b strings.Builder

	options := []string{"Apply to User (all projects)", "Apply to Project..."}
	for i, opt := range options {
		if i == m.applyOptionCursor {
			b.WriteString(styles.ListItemSelected.Render("> " + opt))
		} else {
			b.WriteString(styles.ListItem.Render("  " + opt))
		}
		b.WriteString("\n")
	}

	// Show diff preview for whichever option is highlighted
	if perm != nil {
		b.WriteString("\n")
		if m.applyOptionCursor == 0 {
			// User level preview
			filePath, diffLines, allExist := parser.PreviewUserDiff([]string{perm.Permission.Raw})
			b.WriteString(renderDiffPreview(filePath, diffLines, allExist, 74))
		}
		// Project level shows after project selection, so no preview here
	}

	b.WriteString(fmt.Sprintf("\n%s nav  %s confirm  %s cancel",
		styles.HelpKey.Render("j/k"),
		styles.HelpKey.Render("Enter"),
		styles.HelpKey.Render("Esc")))

	return b.String()
}

func (m Model) renderProjectSelect(perm *types.PermissionStats) string {
	var b strings.Builder
	b.WriteString("  Select project:\n\n")

	maxVisible := 6
	start := 0
	if m.projectListCursor >= maxVisible {
		start = m.projectListCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(perm.Projects) {
		end = len(perm.Projects)
	}

	for i := start; i < end; i++ {
		display := shortenPath(perm.Projects[i])
		if i == m.projectListCursor {
			b.WriteString(styles.ListItemSelected.Render("> " + display))
		} else {
			b.WriteString(styles.ListItem.Render("  " + display))
		}
		b.WriteString("\n")
	}

	if len(perm.Projects) > maxVisible {
		b.WriteString(fmt.Sprintf("\n  (%d/%d)\n", m.projectListCursor+1, len(perm.Projects)))
	}

	// Show diff preview for the selected project
	if m.projectListCursor < len(perm.Projects) {
		b.WriteString("\n")
		projectPath := perm.Projects[m.projectListCursor]
		filePath, diffLines, allExist := parser.PreviewProjectDiff(projectPath, []string{perm.Permission.Raw})
		b.WriteString(renderDiffPreview(filePath, diffLines, allExist, 74))
	}

	b.WriteString(fmt.Sprintf("\n%s nav  %s apply  %s back",
		styles.HelpKey.Render("j/k"),
		styles.HelpKey.Render("Enter"),
		styles.HelpKey.Render("Esc")))

	return b.String()
}

func shortenPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}
