package internal

import tea "github.com/charmbracelet/bubbletea"

// handleMouse processes mouse input
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Ignore mouse in modal
	if m.showApplyModal {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionRelease {
			return m.handleLeftClick(msg)
		}

	case tea.MouseButtonWheelUp:
		m.cursor--
		m.clampCursor()
		return m, nil

	case tea.MouseButtonWheelDown:
		m.cursor++
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleLeftClick processes left mouse clicks
func (m Model) handleLeftClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Check if clicking on tab bar (row 1)
	if msg.Y == 1 {
		return m.handleTabClick(msg)
	}

	// Check if clicking on list items
	// List starts at row 3 (0: title, 1: tabs, 2: header)
	listStartY := 3
	_, contentHeight := m.calculateLayout()

	if msg.Y >= listStartY && msg.Y < listStartY+contentHeight {
		// Calculate which item was clicked
		clickedIndex := msg.Y - listStartY
		perms := m.visiblePermissions()

		if clickedIndex >= 0 && clickedIndex < len(perms) {
			if m.cursor == clickedIndex {
				// Double-click effect: open modal
				m.showApplyModal = true
			} else {
				m.cursor = clickedIndex
			}
		}
	}

	return m, nil
}

// handleTabClick processes clicks on the tab bar
func (m Model) handleTabClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Tab positions (approximate):
	// [Frequency] starts around x=1
	// [Matrix] starts around x=14
	// [Help] starts around x=24

	x := msg.X

	if x >= 1 && x < 13 {
		m.activeView = ViewFrequency
	} else if x >= 13 && x < 23 {
		m.activeView = ViewMatrix
	} else if x >= 23 && x < 31 {
		m.activeView = ViewHelp
	}

	return m, nil
}
