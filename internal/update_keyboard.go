package internal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/b-open-io/claude-perms/internal/parser"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyboard processes keyboard input
func (m Model) handleKeyboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle agent detail modal
	if m.showAgentModal {
		return m.handleAgentModalKeys(msg)
	}

	// Handle apply modal keys
	if m.showApplyModal {
		return m.handleModalKeys(msg)
	}

	// Handle filter mode
	if m.filtering {
		return m.handleFilterKeys(msg)
	}

	// Normal mode keys
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		switch m.activeView {
		case ViewFrequency:
			m.navigateDown()
		case ViewMatrix:
			m.navigateMatrixDown()
		}
		return m, nil

	case "k", "up":
		switch m.activeView {
		case ViewFrequency:
			m.navigateUp()
		case ViewMatrix:
			m.navigateMatrixUp()
		}
		return m, nil

	case "g", "home":
		switch m.activeView {
		case ViewFrequency:
			m.groupCursor = 0
			m.childCursor = -1
		case ViewMatrix:
			m.matrixCursor = 0
			m.matrixScroll = 0
		}
		return m, nil

	case "G", "end":
		switch m.activeView {
		case ViewFrequency:
			m.groupCursor = len(m.permissionGroups) - 1
			if m.groupCursor < 0 {
				m.groupCursor = 0
			}
			m.childCursor = -1
		case ViewMatrix:
			maxIdx := len(m.agents) - 1
			if maxIdx < 0 {
				maxIdx = 0
			}
			m.matrixCursor = maxIdx
		}
		return m, nil

	case "enter", " ":
		switch m.activeView {
		case ViewFrequency:
			// If on a group, toggle expand
			if m.childCursor == -1 && m.groupCursor < len(m.permissionGroups) {
				m.permissionGroups[m.groupCursor].Expanded = !m.permissionGroups[m.groupCursor].Expanded
			}
			// If on a child, show modal
			if m.childCursor >= 0 {
				m.resetApplyModalState()
				m.showApplyModal = true
			}
		case ViewMatrix:
			if len(m.agents) > 0 && m.matrixCursor < len(m.agents) {
				m.selectedAgentIdx = m.matrixCursor
				m.showAgentModal = true
			}
		}
		return m, nil

	case "tab":
		m.activeView = (m.activeView + 1) % 3
		return m, nil

	case "shift+tab":
		m.activeView = (m.activeView + 2) % 3
		return m, nil

	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, nil

	case "esc":
		m.filterInput.SetValue("")
		m.filteredIndices = nil
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleFilterKeys processes keys while in filter mode
func (m Model) handleFilterKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.filtering = false
		m.filterInput.Blur()
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	// Pass to text input
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter()
	return m, cmd
}

// handleModalKeys processes keys while modal is open
func (m Model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.applyModalMode {
	case ApplyModeOptionSelect:
		return m.handleOptionSelectKeys(msg)
	case ApplyModeProjectSelect:
		return m.handleProjectSelectKeys(msg)
	}
	return m, nil
}

func (m Model) handleOptionSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showApplyModal = false
		m.resetApplyModalState()
		return m, nil
	case "j", "down":
		if m.applyOptionCursor < 1 {
			m.applyOptionCursor++
		}
		return m, nil
	case "k", "up":
		if m.applyOptionCursor > 0 {
			m.applyOptionCursor--
		}
		return m, nil
	case "enter", " ":
		if m.applyOptionCursor == 0 {
			return m.applyToUser()
		}
		// Switch to project selection mode
		m.applyModalMode = ApplyModeProjectSelect
		m.projectListCursor = 0
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleProjectSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	perm := m.selectedPermission()
	if perm == nil {
		return m, nil
	}
	maxIdx := len(perm.Projects) - 1

	switch msg.String() {
	case "esc":
		m.applyModalMode = ApplyModeOptionSelect // Back to options
		return m, nil
	case "q":
		m.showApplyModal = false
		m.resetApplyModalState()
		return m, nil
	case "j", "down":
		if m.projectListCursor < maxIdx {
			m.projectListCursor++
		}
		return m, nil
	case "k", "up":
		if m.projectListCursor > 0 {
			m.projectListCursor--
		}
		return m, nil
	case "enter", " ":
		return m.applyToProject()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) applyToUser() (tea.Model, tea.Cmd) {
	perm := m.selectedPermission()
	if perm == nil {
		return m, nil
	}

	if err := parser.WritePermissionToUserSettings(perm.Permission.Raw); err != nil {
		m.err = err
		return m, nil
	}

	m.userApproved = append(m.userApproved, perm.Permission.Raw)
	m.showApplyModal = false
	m.resetApplyModalState()
	return m, nil
}

func (m Model) applyToProject() (tea.Model, tea.Cmd) {
	perm := m.selectedPermission()
	if perm == nil || m.projectListCursor >= len(perm.Projects) {
		return m, nil
	}

	projectPath := perm.Projects[m.projectListCursor]
	if err := parser.WritePermissionToProjectSettings(projectPath, perm.Permission.Raw); err != nil {
		m.err = err
		return m, nil
	}

	m.projectApproved = append(m.projectApproved, perm.Permission.Raw)
	m.showApplyModal = false
	m.resetApplyModalState()
	return m, nil
}

// generateUserCommand creates the command to add permission at user level
func generateUserCommand(permission string) string {
	return fmt.Sprintf(`# Add to ~/.claude/settings.local.json under "permissions.allow":
"%s"`, permission)
}

// generateProjectCommand creates the command to add permission at project level
func generateProjectCommand(permission string) string {
	return fmt.Sprintf(`# Add to .claude/settings.local.json under "permissions.allow":
"%s"`, permission)
}

// copyToClipboard copies text to system clipboard
func copyToClipboard(text string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	default:
		return
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}

	if err := cmd.Start(); err != nil {
		return
	}

	stdin.Write([]byte(text))
	stdin.Close()
	cmd.Wait()
}

// writeToStderr writes a message to stderr (for debugging)
func writeToStderr(msg string) {
	os.Stderr.WriteString(msg + "\n")
}

// handleAgentModalKeys processes keys while agent detail modal is open
func (m Model) handleAgentModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.agentModalMode {
	case AgentModalModePermissions:
		return m.handleAgentPermissionKeys(msg)
	case AgentModalModeScope:
		return m.handleAgentScopeKeys(msg)
	case AgentModalModeProject:
		return m.handleAgentProjectKeys(msg)
	}
	return m, nil
}

func (m Model) handleAgentPermissionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]
	maxIdx := len(agent.Permissions) - 1

	switch msg.String() {
	case "esc", "q":
		m.showAgentModal = false
		m.resetAgentModalState()
		return m, nil

	case "j", "down":
		if m.agentModalCursor < maxIdx {
			m.agentModalCursor++
		}
		return m, nil

	case "k", "up":
		if m.agentModalCursor > 0 {
			m.agentModalCursor--
		}
		return m, nil

	case " ": // Spacebar to toggle
		if m.agentModalCursor <= maxIdx {
			for len(m.agentModalSelected) <= m.agentModalCursor {
				m.agentModalSelected = append(m.agentModalSelected, false)
			}
			m.agentModalSelected[m.agentModalCursor] = !m.agentModalSelected[m.agentModalCursor]
		}
		return m, nil

	case "a", "A": // Apply selected
		hasSelected := false
		for _, sel := range m.agentModalSelected {
			if sel {
				hasSelected = true
				break
			}
		}
		if hasSelected {
			m.agentModalMode = AgentModalModeScope
			m.agentModalScope = 0
		}
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleAgentScopeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.agentModalMode = AgentModalModePermissions
		return m, nil

	case "j", "down":
		if m.agentModalScope < 1 {
			m.agentModalScope++
		}
		return m, nil

	case "k", "up":
		if m.agentModalScope > 0 {
			m.agentModalScope--
		}
		return m, nil

	case "enter", " ":
		if m.agentModalScope == 0 {
			return m.applySelectedToUser()
		} else {
			m.agentModalMode = AgentModalModeProject
			m.agentModalProjCursor = 0
		}
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleAgentProjectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]
	maxIdx := len(agent.Projects) - 1

	switch msg.String() {
	case "esc":
		m.agentModalMode = AgentModalModeScope
		return m, nil

	case "j", "down":
		if m.agentModalProjCursor < maxIdx {
			m.agentModalProjCursor++
		}
		return m, nil

	case "k", "up":
		if m.agentModalProjCursor > 0 {
			m.agentModalProjCursor--
		}
		return m, nil

	case "enter", " ":
		return m.applySelectedToProject()

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) applySelectedToUser() (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]

	for i, perm := range agent.Permissions {
		if i < len(m.agentModalSelected) && m.agentModalSelected[i] {
			if err := parser.WritePermissionToUserSettings(perm.Permission.Raw); err != nil {
				m.err = err
				return m, nil
			}
			m.userApproved = append(m.userApproved, perm.Permission.Raw)
		}
	}

	m.showAgentModal = false
	m.resetAgentModalState()
	return m, nil
}

func (m Model) applySelectedToProject() (tea.Model, tea.Cmd) {
	if m.selectedAgentIdx >= len(m.agentUsage) {
		return m, nil
	}
	agent := m.agentUsage[m.selectedAgentIdx]

	if m.agentModalProjCursor >= len(agent.Projects) {
		return m, nil
	}
	projectPath := agent.Projects[m.agentModalProjCursor]

	for i, perm := range agent.Permissions {
		if i < len(m.agentModalSelected) && m.agentModalSelected[i] {
			if err := parser.WritePermissionToProjectSettings(projectPath, perm.Permission.Raw); err != nil {
				m.err = err
				return m, nil
			}
			m.projectApproved = append(m.projectApproved, perm.Permission.Raw)
		}
	}

	m.showAgentModal = false
	m.resetAgentModalState()
	return m, nil
}

func (m *Model) resetAgentModalState() {
	m.agentModalCursor = 0
	m.agentModalSelected = nil
	m.agentModalMode = 0
	m.agentModalScope = 0
	m.agentModalProjCursor = 0
}
