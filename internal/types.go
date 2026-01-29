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

// ApplyModalMode represents the current mode within the apply modal
type ApplyModalMode int

const (
	ApplyModeOptionSelect  ApplyModalMode = iota // User vs Project selection
	ApplyModeProjectSelect                       // Project dropdown
)

// Model is the main Bubble Tea model
type Model struct {
	// View state
	activeView     ViewType
	showApplyModal bool
	isLoading      bool        // Shows loading indicator during initial data scan
	loadingStatus  string      // Current project path being loaded
	loadingSession string      // Current session ID being scanned
	progressChan   chan string // Channel for streaming progress updates

	// Apply modal state
	applyModalMode    ApplyModalMode
	applyOptionCursor int // 0=User, 1=Project
	projectListCursor int // Index in project list

	// Data
	permissions []types.PermissionStats
	agents      []types.AgentPermissions
	skills      []types.SkillPermissions

	// Agent usage stats (from session logs)
	agentUsage []types.AgentUsageStats

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

	// Hierarchical view (Frequency)
	permissionGroups []types.PermissionGroup
	groupCursor      int // Which group is selected
	childCursor      int // Which child within expanded group (-1 if on group)
	freqScroll       int // Scroll offset for frequency viewport

	// Matrix view state
	matrixCursor     int  // Cursor position in agent/skill list
	matrixScroll     int  // Scroll offset for viewport
	showAgentModal   bool // Show agent detail modal
	selectedAgentIdx int  // Index of agent for detail modal

	// Agent detail modal state
	agentModalCursor    int    // Cursor in permission list
	agentModalSelected  []bool // Which permissions are selected (toggled)
	agentModalMode      int    // 0=permission select, 1=scope select, 2=project select
	agentModalScope     int    // 0=user, 1=project
	agentModalProjCursor int   // Cursor in project list

	// Dimensions
	width  int
	height int

	// Error state
	err error

	// Toast notification (shown after apply)
	toastMessage string // Multi-line message to show
	toastTicks   int    // Remaining ticks before auto-dismiss
}
