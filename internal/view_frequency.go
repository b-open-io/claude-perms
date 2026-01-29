package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/b-open-io/claude-perms/internal/types"
)

// calculateFreqColumns returns responsive column widths for the frequency view.
// Uses weight-based sizing so the permission name column fills available space.
// Returns: allowWidth, denyWidth, permWidth, lastWidth, statusWidth
func (m Model) calculateFreqColumns() (allowWidth, denyWidth, permWidth, lastWidth, statusWidth int) {
	const cursorWidth = 2  // "> " or "  "
	const columnGaps = 8   // 2-space gap between each of the 5 columns (4 gaps * 2)
	const contentPad = 4   // Content area padding

	// Base column widths
	allowWidth = 7  // right-aligned number
	denyWidth = 5   // right-aligned number (typically smaller)
	lastWidth = 10  // relative time
	statusWidth = 8 // "✓ user", "○", etc.

	fixedWidth := cursorWidth + allowWidth + denyWidth + lastWidth + statusWidth + columnGaps + contentPad
	permWidth = m.width - fixedWidth

	// On wide terminals, give data columns more room
	extra := permWidth - 45
	if extra > 0 {
		bonus := extra / 4
		if bonus > 4 {
			bonus = 4
		}
		allowWidth += bonus
		lastWidth += bonus
		statusWidth += bonus
		fixedWidth = cursorWidth + allowWidth + denyWidth + lastWidth + statusWidth + columnGaps + contentPad
		permWidth = m.width - fixedWidth
	}

	if permWidth < 20 {
		permWidth = 20
	}

	return allowWidth, denyWidth, permWidth, lastWidth, statusWidth
}

// freqVisualLine returns the visual line index (0-based) of the current cursor position
// within the flattened list of groups + expanded children.
func (m Model) freqVisualLine() int {
	line := 0
	for gi, group := range m.permissionGroups {
		if gi == m.groupCursor && m.childCursor == -1 {
			return line
		}
		line++ // group header

		if group.Expanded {
			for ci := range group.Children {
				if gi == m.groupCursor && ci == m.childCursor {
					return line
				}
				line++
			}
		}
	}
	return line
}

// freqTotalLines returns the total number of visible lines in the frequency list.
func (m Model) freqTotalLines() int {
	total := 0
	for _, group := range m.permissionGroups {
		total++ // group header
		if group.Expanded {
			total += len(group.Children)
		}
	}
	return total
}

// renderFrequencyView renders the permission frequency list with viewport scrolling
func (m Model) renderFrequencyView() string {
	_, contentHeight := m.calculateLayout()

	// Reserve 2 lines for header and separator
	listHeight := contentHeight - 2
	if listHeight < 1 {
		listHeight = 1
	}

	var lines []string

	// Header
	lines = append(lines, m.renderFrequencyHeader())
	lines = append(lines, styles.ListHeader.Render(strings.Repeat("─", m.width-4)))

	// Build all visible rows, then slice to viewport
	var allRows []string

	for gi, group := range m.permissionGroups {
		isGroupSelected := gi == m.groupCursor && m.childCursor == -1
		expandChar := "▶"
		if group.Expanded {
			expandChar = "▼"
		}

		allRows = append(allRows, m.renderGroupRow(group, expandChar, isGroupSelected))

		if group.Expanded {
			for ci, child := range group.Children {
				isChildSelected := gi == m.groupCursor && ci == m.childCursor
				allRows = append(allRows, m.renderChildRow(child, isChildSelected))
			}
		}
	}

	// Apply scroll offset
	scrollOffset := m.freqScroll
	if scrollOffset > len(allRows) {
		scrollOffset = len(allRows)
	}

	endIdx := scrollOffset + listHeight
	if endIdx > len(allRows) {
		endIdx = len(allRows)
	}

	for i := scrollOffset; i < endIdx; i++ {
		lines = append(lines, allRows[i])
	}

	// Pad remaining lines to fill content area
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines[:contentHeight], "\n") + "\n"
}

// renderFrequencyHeader renders the column headers
func (m Model) renderFrequencyHeader() string {
	allowWidth, denyWidth, permWidth, lastWidth, statusWidth := m.calculateFreqColumns()

	allow := padLeft("Allow", allowWidth)
	deny := padLeft("Deny", denyWidth)
	perm := padRight("Permission", permWidth)
	last := padLeft("Last", lastWidth)
	status := padLeft("Status", statusWidth)

	header := fmt.Sprintf("  %s  %s  %s  %s  %s", allow, deny, perm, last, status)
	header = padRight(header, m.width-4)
	return styles.ListHeader.Render(header)
}

// renderFreqRow builds a frequency row with responsive column widths and full-width padding.
// Status styling is applied AFTER truncation/padding to avoid ANSI escape codes being
// cut mid-sequence by truncateString, which would leak color into subsequent rows.
func (m Model) renderFreqRow(allowText, denyText, permText, timeText, statusText string, selected bool, statusApproved bool) string {
	allowWidth, denyWidth, permWidth, lastWidth, statusWidth := m.calculateFreqColumns()

	allow := padLeft(allowText, allowWidth)
	deny := padLeft(denyText, denyWidth)
	perm := padRight(truncateString(permText, permWidth), permWidth)
	last := padLeft(timeText, lastWidth)
	status := padLeft(statusText, statusWidth)

	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Build row with plain text only — no ANSI codes yet
	row := fmt.Sprintf("%s%s  %s  %s  %s  %s", cursor, allow, deny, perm, last, status)

	// Truncate and pad using plain byte lengths (safe since no ANSI codes)
	maxWidth := m.width - 2
	row = truncateString(row, maxWidth)
	row = padRight(row, maxWidth)

	// Now apply coloring. Replace the plain status text at the end with styled version.
	plainStatus := padLeft(statusText, statusWidth)
	var styledStatus string
	if statusApproved {
		styledStatus = styles.StatusApproved.Render(plainStatus)
	} else {
		styledStatus = styles.StatusPending.Render(plainStatus)
	}
	// Find the last occurrence of the plain status and replace it with styled
	idx := strings.LastIndex(row, plainStatus)
	if idx >= 0 {
		row = row[:idx] + styledStatus + row[idx+len(plainStatus):]
	}

	if selected {
		return styles.ListItemSelected.Render(row)
	}
	return row
}

func (m Model) renderGroupRow(g types.PermissionGroup, expandChar string, selected bool) string {
	allowText := fmt.Sprintf("%d", g.TotalApproved)
	denyText := fmt.Sprintf("%d", g.TotalDenied)

	name := fmt.Sprintf("%s %s", expandChar, g.Type)
	if len(g.Children) > 1 {
		name += fmt.Sprintf(" (%d variants)", len(g.Children))
	}

	timeText := formatRelativeTime(g.LastSeen)
	approved := g.ApprovedAt > types.NotApproved

	var statusText string
	if approved {
		statusText = g.ApprovedAt.String()
	} else {
		statusText = "○"
	}

	return m.renderFreqRow(allowText, denyText, name, timeText, statusText, selected, approved)
}

func (m Model) renderChildRow(p types.PermissionStats, selected bool) string {
	allowText := fmt.Sprintf("%d", p.Approved)
	denyText := fmt.Sprintf("%d", p.Denied)
	name := "    " + p.Permission.Raw
	timeText := formatRelativeTime(p.LastSeen)
	approved := p.ApprovedAt > types.NotApproved

	var statusText string
	if approved {
		statusText = p.ApprovedAt.String()
	} else {
		statusText = "○"
	}

	return m.renderFreqRow(allowText, denyText, name, timeText, statusText, selected, approved)
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
