package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/b-open-io/claude-perms/internal/types"
)

// renderFrequencyView renders the permission frequency list
func (m Model) renderFrequencyView() string {
	_, contentHeight := m.calculateLayout()
	perms := m.visiblePermissions()

	var lines []string

	// Header row
	header := renderFrequencyHeader()
	lines = append(lines, header)

	// Separator
	lines = append(lines, styles.ListHeader.Render(strings.Repeat("─", m.width-4)))

	// Permission rows
	for i, p := range perms {
		if i >= contentHeight-2 { // Account for header and separator
			break
		}

		selected := i == m.cursor
		line := renderFrequencyRow(p, selected, m.width)
		lines = append(lines, line)
	}

	// Pad remaining lines
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines[:contentHeight], "\n") + "\n"
}

// renderFrequencyHeader renders the column headers
func renderFrequencyHeader() string {
	count := styles.ColCount.Render("Count")
	perm := styles.ColPerm.Render("Permission")
	last := styles.ColTime.Render("Last")
	status := styles.ColStatus.Render("Status")

	return styles.ListHeader.Render(fmt.Sprintf("%s  %s  %s  %s", count, perm, last, status))
}

// renderFrequencyRow renders a single permission row
func renderFrequencyRow(p types.PermissionStats, selected bool, width int) string {
	// Format count
	count := fmt.Sprintf("%d", p.Count)
	countStyled := styles.ColCount.Render(count)

	// Format permission (truncate if needed)
	maxPermWidth := 35
	permText := truncateString(p.Permission.Raw, maxPermWidth)
	permStyled := styles.ColPerm.Render(permText)

	// Format time
	timeText := formatRelativeTime(p.LastSeen)
	timeStyled := styles.ColTime.Render(timeText)

	// Format status
	var statusStyled string
	switch p.ApprovedAt {
	case types.ApprovedUser, types.ApprovedProject:
		statusStyled = styles.StatusApproved.Render(p.ApprovedAt.String())
	default:
		statusStyled = styles.StatusPending.Render(p.ApprovedAt.String())
	}

	row := fmt.Sprintf("%s  %s  %s  %s", countStyled, permStyled, timeStyled, statusStyled)

	if selected {
		// Add selection indicator and highlight
		return styles.ListItemSelected.Render("> " + row)
	}

	return styles.ListItem.Render(row)
}

// formatRelativeTime formats a time as relative (e.g., "2h ago", "3d ago")
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		return fmt.Sprintf("%dw ago", weeks)
	default:
		months := int(diff.Hours() / 24 / 30)
		if months < 1 {
			months = 1
		}
		return fmt.Sprintf("%dmo ago", months)
	}
}
