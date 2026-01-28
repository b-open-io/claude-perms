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
	FilePath    string
	Permissions []Permission
}

// SkillPermissions holds permissions declared by a skill
type SkillPermissions struct {
	Name        string
	Plugin      string
	FilePath    string
	Permissions []Permission
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
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	AllowedTools []string `yaml:"allowedTools"`
}

// SkillFrontmatter represents the YAML frontmatter of a skill file
type SkillFrontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	AllowedTools []string `yaml:"allowedTools"`
}

// PermissionGroup represents a permission type with its children
type PermissionGroup struct {
	Type       string            // "Bash", "Read", etc.
	TotalCount int               // Sum of all children counts
	LastSeen   time.Time         // Most recent across all children
	Children   []PermissionStats // Individual permissions like Bash(curl:*)
	Expanded   bool              // UI state: is this group expanded?
	ApprovedAt ApprovalLevel     // Highest approval level among children
}
