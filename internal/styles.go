package internal

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	ColorPrimary   = lipgloss.Color("12")  // Blue
	ColorSecondary = lipgloss.Color("244") // Gray
	ColorSuccess   = lipgloss.Color("10")  // Green
	ColorWarning   = lipgloss.Color("11")  // Yellow
	ColorMuted     = lipgloss.Color("240") // Dark gray
	ColorHighlight = lipgloss.Color("14")  // Cyan
)

// Styles holds all the lipgloss styles for the TUI
type Styles struct {
	// Layout
	App         lipgloss.Style
	TitleBar    lipgloss.Style
	StatusBar   lipgloss.Style
	Content     lipgloss.Style

	// Tabs
	Tab         lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style

	// List
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListHeader       lipgloss.Style

	// Columns
	ColCount    lipgloss.Style
	ColPerm     lipgloss.Style
	ColTime     lipgloss.Style
	ColStatus   lipgloss.Style

	// Modal
	Modal       lipgloss.Style
	ModalTitle  lipgloss.Style
	ModalBox    lipgloss.Style

	// Status indicators
	StatusApproved lipgloss.Style
	StatusPending  lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
}

// DefaultStyles returns the default style configuration
func DefaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle(),

		TitleBar: lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			Background(ColorPrimary).
			Foreground(lipgloss.Color("15")),

		StatusBar: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Padding(0, 1),

		Content: lipgloss.NewStyle().
			Padding(0, 1),

		Tab: lipgloss.NewStyle().
			Padding(0, 2),

		TabActive: lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(ColorPrimary).
			Underline(true),

		TabInactive: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(ColorSecondary),

		ListItem: lipgloss.NewStyle().
			PaddingLeft(2),

		ListItemSelected: lipgloss.NewStyle().
			PaddingLeft(0).
			Foreground(ColorHighlight).
			Bold(true),

		ListHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary).
			PaddingLeft(2),

		ColCount: lipgloss.NewStyle().
			Width(7).
			Align(lipgloss.Right),

		ColPerm: lipgloss.NewStyle().
			Width(35),

		ColTime: lipgloss.NewStyle().
			Width(10).
			Foreground(ColorMuted),

		ColStatus: lipgloss.NewStyle().
			Width(8),

		Modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2),

		ModalTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1),

		ModalBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorSecondary).
			Padding(0, 1).
			MarginTop(1),

		StatusApproved: lipgloss.NewStyle().
			Foreground(ColorSuccess),

		StatusPending: lipgloss.NewStyle().
			Foreground(ColorMuted),

		HelpKey: lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(ColorSecondary),
	}
}

// Global styles instance
var styles = DefaultStyles()

// GetStyles returns the global styles
func GetStyles() Styles {
	return styles
}
