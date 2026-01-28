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
		activeView:       ViewFrequency,
		showApplyModal:   false,
		isLoading:        true,
		permissions:      nil,
		permissionGroups: nil,
		agents:           nil,
		skills:           nil,
		userApproved:     nil,
		projectApproved:  nil,
		projectPath:      cwd,
		cursor:           0,
		groupCursor:      0,
		childCursor:      -1, // Start on group, not child
		filterInput:      ti,
		filtering:        false,
		filteredIndices:  nil,
		width:            80,
		height:           24,
		err:              nil,
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

// loadingProgressMsg updates the loading status display
type loadingProgressMsg struct {
	status string
}

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

// LoadData loads all permission data from disk with progress updates
func LoadData(projectPath string, progress chan<- string) dataLoadedMsg {
	// Load permission stats from session logs with progress
	permissions, err := parser.LoadAllPermissionStatsWithProgress(progress)
	if err != nil {
		return dataLoadedMsg{err: err}
	}

	// Load approved permissions
	if progress != nil {
		progress <- "Loading user settings..."
	}
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
	if progress != nil {
		progress <- "Grouping permissions..."
	}
	groups := parser.GroupPermissions(permissions)

	// Load agents and skills
	if progress != nil {
		progress <- "Loading agents..."
	}
	agents, _ := parser.LoadAllAgents()

	if progress != nil {
		progress <- "Loading skills..."
	}
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

// progressReader returns a command that reads from the progress channel
func progressReader(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		status, ok := <-ch
		if !ok {
			return nil // Channel closed, ignore
		}
		return loadingProgressMsg{status: status}
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
	if len(m.permissionGroups) == 0 {
		return nil
	}
	if m.groupCursor >= len(m.permissionGroups) {
		return nil
	}

	group := &m.permissionGroups[m.groupCursor]

	if m.childCursor >= 0 && m.childCursor < len(group.Children) {
		// Return specific child
		return &group.Children[m.childCursor]
	}

	// Return first child of group (or nil if no children)
	if len(group.Children) > 0 {
		return &group.Children[0]
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

func (m *Model) navigateDown() {
	if len(m.permissionGroups) == 0 {
		return
	}

	group := &m.permissionGroups[m.groupCursor]

	if m.childCursor == -1 {
		// On group header
		if group.Expanded && len(group.Children) > 0 {
			// Move into children
			m.childCursor = 0
		} else {
			// Move to next group
			if m.groupCursor < len(m.permissionGroups)-1 {
				m.groupCursor++
			}
		}
	} else {
		// In children
		if m.childCursor < len(group.Children)-1 {
			m.childCursor++
		} else {
			// Move to next group
			if m.groupCursor < len(m.permissionGroups)-1 {
				m.groupCursor++
				m.childCursor = -1
			}
		}
	}
}

func (m *Model) navigateUp() {
	if len(m.permissionGroups) == 0 {
		return
	}

	if m.childCursor == -1 {
		// On group header
		if m.groupCursor > 0 {
			m.groupCursor--
			prevGroup := &m.permissionGroups[m.groupCursor]
			if prevGroup.Expanded && len(prevGroup.Children) > 0 {
				m.childCursor = len(prevGroup.Children) - 1
			}
		}
	} else {
		// In children
		if m.childCursor > 0 {
			m.childCursor--
		} else {
			m.childCursor = -1 // Back to group header
		}
	}
}
