package internal

import (
	"fmt"
	"os"
	"time"

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

// toastTickMsg is sent to count down the toast display timer
type toastTickMsg struct{}

// toastTickCmd returns a command that sends a toastTickMsg after a delay
func toastTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return toastTickMsg{}
	})
}

// setApplyToast sets the toast message from an apply result
func (m *Model) setApplyToast(result *parser.ApplyResult) {
	if !result.WasNew {
		m.toastMessage = fmt.Sprintf("Already exists in %s", result.FilePath)
		m.toastTicks = 3
		return
	}
	if result.LineNumber > 0 {
		m.toastMessage = fmt.Sprintf("Written to %s:%d", result.FilePath, result.LineNumber)
	} else {
		m.toastMessage = fmt.Sprintf("Written to %s", result.FilePath)
	}
	m.toastTicks = 4
}

// setApplyToastBatch sets a toast for batch apply
func (m *Model) setApplyToastBatch(filePath string, count int) {
	m.toastMessage = fmt.Sprintf("%d permissions written to %s", count, filePath)
	m.toastTicks = 4
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
	agentUsage       []types.AgentUsageStats
	userApproved     []string
	projectApproved  []string
	err              error
}

// LoadData loads all permission data from disk with progress updates
func LoadData(projectPath string, progress chan<- string) dataLoadedMsg {
	// Load permission stats from session logs with caching
	permissions, err := parser.LoadAllPermissionStatsWithCache(progress)
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

	// Load agent usage stats from session logs
	agentUsage, _ := parser.LoadAgentUsageStats(progress)

	return dataLoadedMsg{
		permissions:      permissions,
		permissionGroups: groups,
		agents:           agents,
		skills:           skills,
		agentUsage:       agentUsage,
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
func (m Model) calculateLayout() (contentWidth, contentHeight int) {
	contentWidth = m.width
	contentHeight = m.height

	// Subtract fixed UI elements
	contentHeight -= 1 // title bar
	contentHeight -= 1 // tab bar
	contentHeight -= 1 // status bar

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
		if group.Expanded && len(group.Children) > 0 {
			m.childCursor = 0
		} else {
			if m.groupCursor < len(m.permissionGroups)-1 {
				m.groupCursor++
			}
		}
	} else {
		if m.childCursor < len(group.Children)-1 {
			m.childCursor++
		} else {
			if m.groupCursor < len(m.permissionGroups)-1 {
				m.groupCursor++
				m.childCursor = -1
			}
		}
	}

	m.updateFreqScroll()
}

func (m *Model) navigateUp() {
	if len(m.permissionGroups) == 0 {
		return
	}

	if m.childCursor == -1 {
		if m.groupCursor > 0 {
			m.groupCursor--
			prevGroup := &m.permissionGroups[m.groupCursor]
			if prevGroup.Expanded && len(prevGroup.Children) > 0 {
				m.childCursor = len(prevGroup.Children) - 1
			}
		}
	} else {
		if m.childCursor > 0 {
			m.childCursor--
		} else {
			m.childCursor = -1
		}
	}

	m.updateFreqScroll()
}

// updateFreqScroll ensures the cursor is visible within the frequency viewport.
func (m *Model) updateFreqScroll() {
	_, contentHeight := m.calculateLayout()
	viewportHeight := contentHeight - 2 // header + separator
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	cursorLine := m.freqVisualLine()

	// Scroll down if cursor is below viewport
	if cursorLine >= m.freqScroll+viewportHeight {
		m.freqScroll = cursorLine - viewportHeight + 1
	}
	// Scroll up if cursor is above viewport
	if cursorLine < m.freqScroll {
		m.freqScroll = cursorLine
	}
}

// resetApplyModalState resets modal to initial state
func (m *Model) resetApplyModalState() {
	m.applyModalMode = ApplyModeOptionSelect
	m.applyOptionCursor = 0
	m.projectListCursor = 0
}

// navigateMatrixDown moves cursor down in Matrix view
func (m *Model) navigateMatrixDown() {
	maxIdx := m.matrixListLen() - 1
	if maxIdx < 0 {
		return
	}

	if m.matrixCursor < maxIdx {
		m.matrixCursor++

		// Scroll if needed
		_, contentHeight := m.calculateLayout()
		viewportHeight := contentHeight - 5
		if m.matrixCursor >= m.matrixScroll+viewportHeight {
			m.matrixScroll++
		}
	}
}

// matrixListLen returns the length of the active matrix list
func (m *Model) matrixListLen() int {
	if len(m.agentUsage) > 0 {
		return len(m.agentUsage)
	}
	return len(m.agents)
}

// navigateMatrixUp moves cursor up in Matrix view
func (m *Model) navigateMatrixUp() {
	if m.matrixCursor > 0 {
		m.matrixCursor--

		// Scroll if needed
		if m.matrixCursor < m.matrixScroll {
			m.matrixScroll = m.matrixCursor
		}
	}
}
