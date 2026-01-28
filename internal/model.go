package internal

import (
	"os"

	"github.com/b-open-io/claude-perms/internal/parser"
	"github.com/b-open-io/claude-perms/internal/types"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// NewModel creates and initializes a new Model
func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Filter..."
	ti.CharLimit = 50

	// Get current working directory for project context
	cwd, _ := os.Getwd()

	return Model{
		activeView:      ViewFrequency,
		showApplyModal:  false,
		permissions:     nil,
		agents:          nil,
		skills:          nil,
		userApproved:    nil,
		projectApproved: nil,
		projectPath:     cwd,
		cursor:          0,
		filterInput:     ti,
		filtering:       false,
		filteredIndices: nil,
		width:           80,
		height:          24,
		err:             nil,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadDataCmd,
		tea.EnterAltScreen,
	)
}

// loadDataCmd loads all permission data
func loadDataCmd() tea.Msg {
	return loadDataMsg{}
}

// loadDataMsg triggers data loading
type loadDataMsg struct{}

// dataLoadedMsg contains loaded data
type dataLoadedMsg struct {
	permissions      []types.PermissionStats
	permissionGroups []types.PermissionGroup
	agents           []types.AgentPermissions
	skills           []types.SkillPermissions
	userApproved     []string
	projectApproved  []string
	err              error
}

// LoadData loads all permission data from disk
func LoadData(projectPath string) dataLoadedMsg {
	// Load permission stats from session logs
	permissions, err := parser.LoadAllPermissionStats()
	if err != nil {
		return dataLoadedMsg{err: err}
	}

	// Load approved permissions
	userApproved, _ := parser.LoadUserSettings()
	projectApproved, _ := parser.LoadProjectSettings(projectPath)

	// Update approval status for each permission
	for i := range permissions {
		permissions[i].ApprovedAt = parser.GetApprovalLevel(
			permissions[i].Permission.Raw,
			userApproved,
			projectApproved,
		)
	}

	// Group permissions by type
	groups := parser.GroupPermissions(permissions)

	// Load agents and skills
	agents, _ := parser.LoadAllAgents()
	skills, _ := parser.LoadAllSkills()

	return dataLoadedMsg{
		permissions:      permissions,
		permissionGroups: groups,
		agents:           agents,
		skills:           skills,
		userApproved:     userApproved,
		projectApproved:  projectApproved,
		err:              nil,
	}
}

// calculateLayout returns the available content dimensions
// Following Golden Rule #1: Always account for borders
func (m Model) calculateLayout() (contentWidth, contentHeight int) {
	contentWidth = m.width
	contentHeight = m.height

	// Subtract UI elements
	contentHeight -= 1 // title bar
	contentHeight -= 1 // tab bar
	contentHeight -= 1 // header row
	contentHeight -= 1 // status bar
	contentHeight -= 2 // CRITICAL: panel borders (Golden Rule #1)

	if contentHeight < 1 {
		contentHeight = 1
	}

	return contentWidth, contentHeight
}

// visiblePermissions returns the permissions to display (filtered or all)
func (m Model) visiblePermissions() []types.PermissionStats {
	if len(m.filteredIndices) > 0 {
		result := make([]types.PermissionStats, len(m.filteredIndices))
		for i, idx := range m.filteredIndices {
			if idx < len(m.permissions) {
				result[i] = m.permissions[idx]
			}
		}
		return result
	}
	return m.permissions
}

// selectedPermission returns the currently selected permission
func (m Model) selectedPermission() *types.PermissionStats {
	perms := m.visiblePermissions()
	if m.cursor >= 0 && m.cursor < len(perms) {
		return &perms[m.cursor]
	}
	return nil
}

// clampCursor ensures cursor is within valid bounds
func (m *Model) clampCursor() {
	perms := m.visiblePermissions()
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(perms) {
		m.cursor = len(perms) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
