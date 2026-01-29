package types

import "time"

// Permission represents a parsed permission string
type Permission struct {
	Type  string // "Bash", "Skill", "WebFetch", "Write", etc.
	Scope string // "curl:*", "domain:github.com", etc. (empty if no scope)
	Raw   string // Original string: "Bash(curl:*)"
}

// PermissionStats tracks usage statistics for a permission
type PermissionStats struct {
	Permission Permission
	Count      int
	Approved   int // tool_results where is_error != true
	Denied     int // tool_results where user rejected
	LastSeen   time.Time
	Projects   []string // Project paths where this permission was requested
	ApprovedAt ApprovalLevel
}

// ApprovalLevel indicates where a permission is approved
type ApprovalLevel int

const (
	NotApproved ApprovalLevel = iota
	ApprovedProject
	ApprovedUser
)

func (a ApprovalLevel) String() string {
	switch a {
	case ApprovedUser:
		return "✓ user"
	case ApprovedProject:
		return "✓ proj"
	default:
		return "○"
	}
}

// AgentPermissions holds permissions declared by an agent
type AgentPermissions struct {
	Name        string
	Plugin      string // Plugin name if from a plugin, empty otherwise
	Version     string // Plugin version (e.g., "1.0.20")
	FilePath    string
	Permissions []Permission
}

// SkillPermissions holds permissions declared by a skill
type SkillPermissions struct {
	Name        string
	Plugin      string
	Version     string // Plugin version (e.g., "1.0.20")
	FilePath    string
	Permissions []Permission
}

// AgentUsageStats tracks actual permission usage by an agent type
type AgentUsageStats struct {
	AgentType   string            // "Explore", "bopen-tools:devops-specialist", etc.
	Permissions []PermissionStats // Actual tool_uses from this agent
	TotalCalls  int               // Sum of all permission counts
	LastSeen    time.Time         // Most recent activity
	Sessions    int               // Number of sessions this agent ran in
	Projects    []string          // Projects where this agent was used
}

// Settings represents the settings.local.json structure
type Settings struct {
	Permissions PermissionSettings `json:"permissions"`
}

// PermissionSettings holds the permissions configuration
type PermissionSettings struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// AgentFrontmatter represents the YAML frontmatter of an agent file
type AgentFrontmatter struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tools       interface{} `yaml:"tools"` // Can be []string or comma-separated string
}

// SkillFrontmatter represents the YAML frontmatter of a skill file
type SkillFrontmatter struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tools       interface{} `yaml:"tools"` // Can be []string or comma-separated string
}

// PermissionGroup represents a permission type with its children
type PermissionGroup struct {
	Type          string            // "Bash", "Read", etc.
	TotalCount    int               // Sum of all children counts
	TotalApproved int               // Sum of all children approved counts
	TotalDenied   int               // Sum of all children denied counts
	LastSeen      time.Time         // Most recent across all children
	Children      []PermissionStats // Individual permissions like Bash(curl:*)
	Expanded      bool              // UI state: is this group expanded?
	ApprovedAt    ApprovalLevel     // Highest approval level among children
}
