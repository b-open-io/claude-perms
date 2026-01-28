package internal

import (
	"github.com/b-open-io/claude-perms/internal/types"
	"github.com/charmbracelet/bubbles/textinput"
)

// ViewType represents the active view in the TUI
type ViewType int

const (
	ViewFrequency ViewType = iota
	ViewMatrix
	ViewHelp
)

// Model is the main Bubble Tea model
type Model struct {
	// View state
	activeView     ViewType
	showApplyModal bool

	// Data
	permissions []types.PermissionStats
	agents      []types.AgentPermissions
	skills      []types.SkillPermissions

	// Approved permissions from settings
	userApproved    []string
	projectApproved []string

	// Current project path
	projectPath string

	// UI state
	cursor      int
	filterInput textinput.Model
	filtering   bool

	// Filtered list
	filteredIndices []int

	// Dimensions
	width  int
	height int

	// Error state
	err error
}
