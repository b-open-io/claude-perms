package parser

import (
	"encoding/json"
	"strings"
)

// BashInput represents the input structure for Bash tool_use
type BashInput struct {
	Command string `json:"command"`
}

// SkillInput represents the input structure for Skill tool_use
type SkillInput struct {
	Skill string `json:"skill"`
}

// ExtractPermissionScope extracts the full permission string from tool_use data
// e.g., "Bash" + {"command": "curl https://..."} -> "Bash(curl:*)"
func ExtractPermissionScope(toolName string, inputJSON json.RawMessage) string {
	if len(inputJSON) == 0 {
		return toolName
	}

	switch toolName {
	case "Bash":
		var input BashInput
		if err := json.Unmarshal(inputJSON, &input); err == nil && input.Command != "" {
			cmd := extractBashCommand(input.Command)
			if cmd != "" {
				return "Bash(" + cmd + ":*)"
			}
		}
	case "Skill":
		var input SkillInput
		if err := json.Unmarshal(inputJSON, &input); err == nil && input.Skill != "" {
			return "Skill(" + input.Skill + ")"
		}
		// Read, Write, Edit, Glob, Grep, etc. don't have scopes in settings.json format
	}

	return toolName
}

// extractBashCommand extracts the command name from a bash command string
// "curl https://api.example.com" -> "curl"
// "git -C /path status" -> "git"
// "go build ./..." -> "go build"
func extractBashCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	// Split by spaces and get first word
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	firstWord := parts[0]

	// Handle compound commands: "go build", "npm run", "bun run", etc.
	compoundCommands := map[string]bool{
		"go": true, "npm": true, "bun": true, "yarn": true,
		"cargo": true, "git": true, "docker": true,
	}

	if compoundCommands[firstWord] && len(parts) >= 2 {
		// Don't include flags as part of compound command
		secondWord := parts[1]
		if !strings.HasPrefix(secondWord, "-") {
			return firstWord + " " + secondWord
		}
	}

	return firstWord
}
