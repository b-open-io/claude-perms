package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/b-open-io/claude-perms/internal/types"
	"gopkg.in/yaml.v3"
)

// LoadAllAgents loads agent permissions from all sources
func LoadAllAgents() ([]types.AgentPermissions, error) {
	var agents []types.AgentPermissions

	// Load from ~/.claude/agents/
	userAgents, err := loadAgentsFromDir(filepath.Join(claudeDir(), "agents"), "")
	if err == nil {
		agents = append(agents, userAgents...)
	}

	// Load from plugins
	pluginAgents, err := loadAgentsFromPlugins()
	if err == nil {
		agents = append(agents, pluginAgents...)
	}

	return agents, nil
}

// loadAgentsFromDir loads all agents from a directory
func loadAgentsFromDir(dir, pluginName string) ([]types.AgentPermissions, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var agents []types.AgentPermissions

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		agent, err := parseAgentFile(path, pluginName)
		if err != nil {
			continue
		}

		if len(agent.Permissions) > 0 {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

// loadAgentsFromPlugins loads agents from installed plugins
func loadAgentsFromPlugins() ([]types.AgentPermissions, error) {
	cacheDir := filepath.Join(claudeDir(), "plugins", "cache")

	var agents []types.AgentPermissions

	// Walk the cache directory structure: cache/<marketplace>/<plugin>/<version>/
	marketplaces, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, err
	}

	for _, marketplace := range marketplaces {
		if !marketplace.IsDir() {
			continue
		}

		marketplacePath := filepath.Join(cacheDir, marketplace.Name())
		plugins, err := os.ReadDir(marketplacePath)
		if err != nil {
			continue
		}

		for _, plugin := range plugins {
			if !plugin.IsDir() {
				continue
			}

			pluginPath := filepath.Join(marketplacePath, plugin.Name())
			versions, err := os.ReadDir(pluginPath)
			if err != nil {
				continue
			}

			// Use the latest version (last in sorted order)
			for _, version := range versions {
				if !version.IsDir() {
					continue
				}

				versionPath := filepath.Join(pluginPath, version.Name())
				agentsDir := filepath.Join(versionPath, "agents")

				pluginAgents, err := loadAgentsFromDir(agentsDir, plugin.Name())
				if err == nil {
					agents = append(agents, pluginAgents...)
				}
			}
		}
	}

	return agents, nil
}

// parseAgentFile parses an agent markdown file and extracts frontmatter
func parseAgentFile(path, pluginName string) (types.AgentPermissions, error) {
	file, err := os.Open(path)
	if err != nil {
		return types.AgentPermissions{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Read until we find frontmatter delimiter
	var inFrontmatter bool
	var frontmatterLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				// End of frontmatter
				break
			}
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		}
	}

	if len(frontmatterLines) == 0 {
		return types.AgentPermissions{}, nil
	}

	// Parse YAML
	var frontmatter types.AgentFrontmatter
	yamlContent := strings.Join(frontmatterLines, "\n")
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return types.AgentPermissions{}, err
	}

	// Determine agent name
	name := frontmatter.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	return types.AgentPermissions{
		Name:        name,
		Plugin:      pluginName,
		FilePath:    path,
		Permissions: ParsePermissions(frontmatter.AllowedTools),
	}, nil
}
