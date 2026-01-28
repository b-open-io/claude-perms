package internal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyboard processes keyboard input
func (m Model) handleKeyboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle modal keys first
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
		m.navigateDown()
		return m, nil

	case "k", "up":
		m.navigateUp()
		return m, nil

	case "g", "home":
		m.cursor = 0
		return m, nil

	case "G", "end":
		perms := m.visiblePermissions()
		m.cursor = len(perms) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	case "enter", " ":
		// If on a group, toggle expand
		if m.childCursor == -1 && m.groupCursor < len(m.permissionGroups) {
			m.permissionGroups[m.groupCursor].Expanded = !m.permissionGroups[m.groupCursor].Expanded
		}
		// If on a child, show modal
		if m.childCursor >= 0 {
			m.showApplyModal = true
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
	switch msg.String() {
	case "esc", "q":
		m.showApplyModal = false
		return m, nil

	case "u":
		// Copy user-level command
		if perm := m.selectedPermission(); perm != nil {
			cmd := generateUserCommand(perm.Permission.Raw)
			copyToClipboard(cmd)
		}
		m.showApplyModal = false
		return m, nil

	case "p":
		// Copy project-level command
		if perm := m.selectedPermission(); perm != nil {
			cmd := generateProjectCommand(perm.Permission.Raw)
			copyToClipboard(cmd)
		}
		m.showApplyModal = false
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

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
