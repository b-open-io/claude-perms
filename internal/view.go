package internal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	var b strings.Builder

	// Title bar
	b.WriteString(m.renderTitleBar())
	b.WriteString("\n")

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Main content based on active view
	switch m.activeView {
	case ViewFrequency:
		b.WriteString(m.renderFrequencyView())
	case ViewMatrix:
		b.WriteString(m.renderMatrixView())
	case ViewHelp:
		b.WriteString(m.renderHelpView())
	}

	// Status bar
	b.WriteString(m.renderStatusBar())

	// Modal overlay
	if m.showApplyModal {
		return m.renderWithModal(b.String())
	}

	return b.String()
}

// renderTitleBar renders the title bar
func (m Model) renderTitleBar() string {
	s := styles.TitleBar
	title := "Permission Analyzer"

	// Fill to width
	padding := m.width - len(title) - 2
	if padding < 0 {
		padding = 0
	}

	return s.Render(title + strings.Repeat(" ", padding))
}

// renderTabBar renders the tab navigation
func (m Model) renderTabBar() string {
	tabs := []string{"Frequency", "Matrix", "Help"}
	var parts []string

	for i, tab := range tabs {
		var style lipgloss.Style
		if ViewType(i) == m.activeView {
			style = styles.TabActive
		} else {
			style = styles.TabInactive
		}
		parts = append(parts, style.Render("["+tab+"]"))
	}

	return strings.Join(parts, " ")
}

// renderStatusBar renders the bottom status bar
func (m Model) renderStatusBar() string {
	var left, right string

	if m.filtering {
		left = "Filter: " + m.filterInput.View()
	} else {
		perms := m.visiblePermissions()
		if len(perms) > 0 {
			left = fmt.Sprintf("%d/%d permissions", m.cursor+1, len(perms))
		} else {
			left = "No permissions found"
		}
	}

	right = "j/k: nav  Enter: apply  Tab: view  /: filter  q: quit"

	// Calculate spacing
	spacing := m.width - len(left) - len(right) - 2
	if spacing < 1 {
		spacing = 1
	}

	return styles.StatusBar.Render(left + strings.Repeat(" ", spacing) + right)
}

// renderHelpView renders the help view
func (m Model) renderHelpView() string {
	_, contentHeight := m.calculateLayout()

	help := []struct{ key, desc string }{
		{"j/k, ↓/↑", "Navigate list"},
		{"g/G", "Go to first/last"},
		{"Enter", "Open apply modal"},
		{"Tab", "Switch views"},
		{"/", "Filter permissions"},
		{"Esc", "Clear filter"},
		{"q", "Quit"},
		{"", ""},
		{"In modal:", ""},
		{"u", "Copy user-level command"},
		{"p", "Copy project-level command"},
		{"Esc", "Close modal"},
	}

	var lines []string
	for _, h := range help {
		if h.key == "" {
			lines = append(lines, "")
		} else {
			key := styles.HelpKey.Render(fmt.Sprintf("%12s", h.key))
			desc := styles.HelpDesc.Render("  " + h.desc)
			lines = append(lines, key+desc)
		}
	}

	// Pad to content height
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines[:contentHeight], "\n") + "\n"
}

// renderWithModal overlays the modal on top of the main content
func (m Model) renderWithModal(background string) string {
	// Instead of overlay, just center the modal in available space
	modal := m.renderApplyModal()

	// Calculate centering
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)

	// Pad vertically to center
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

// truncateString truncates a string to maxLen characters
// Following Golden Rule #2: Never auto-wrap in bordered panels
func truncateString(s string, maxLen int) string {
	if maxLen < 1 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 2 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

// padRight pads a string to the specified width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
