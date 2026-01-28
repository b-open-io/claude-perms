package internal

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

var debugLog *os.File

func init() {
	var err error
	debugLog, err = os.OpenFile("/tmp/perms-debug.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(debugLog)
	}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("Update received: %T", msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Printf("WindowSize: %dx%d", msg.Width, msg.Height)
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case loadDataMsg:
		log.Printf("loadDataMsg received, loading data...")
		return m, func() tea.Msg {
			return LoadData(m.projectPath)
		}

	case dataLoadedMsg:
		log.Printf("dataLoadedMsg: err=%v, perms=%d, groups=%d, agents=%d, skills=%d",
			msg.err, len(msg.permissions), len(msg.permissionGroups), len(msg.agents), len(msg.skills))
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.permissions = msg.permissions
		m.permissionGroups = msg.permissionGroups
		m.agents = msg.agents
		m.skills = msg.skills
		m.userApproved = msg.userApproved
		m.projectApproved = msg.projectApproved
		m.clampCursor()
		log.Printf("Model updated: %d permissions, %d groups loaded", len(m.permissions), len(m.permissionGroups))
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyboard(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	// Handle text input updates when filtering
	if m.filtering {
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.applyFilter()
		return m, cmd
	}

	return m, nil
}

// applyFilter filters permissions based on current filter input
func (m *Model) applyFilter() {
	query := m.filterInput.Value()
	if query == "" {
		m.filteredIndices = nil
		m.clampCursor()
		return
	}

	m.filteredIndices = nil
	for i, p := range m.permissions {
		if containsIgnoreCase(p.Permission.Raw, query) {
			m.filteredIndices = append(m.filteredIndices, i)
		}
	}
	m.cursor = 0
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

// toLower converts string to lowercase (simple ASCII)
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
