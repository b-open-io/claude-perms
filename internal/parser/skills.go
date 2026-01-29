package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/b-open-io/claude-perms/internal/types"
	"gopkg.in/yaml.v3"
)

// LoadAllSkills loads skill permissions from all sources
func LoadAllSkills() ([]types.SkillPermissions, error) {
	var skills []types.SkillPermissions

	// Load from plugins
	pluginSkills, err := loadSkillsFromPlugins()
	if err == nil {
		skills = append(skills, pluginSkills...)
	}

	return skills, nil
}

// loadSkillsFromPlugins loads skills from installed plugins
func loadSkillsFromPlugins() ([]types.SkillPermissions, error) {
	cacheDir := filepath.Join(claudeDir(), "plugins", "cache")

	var skills []types.SkillPermissions

	// Walk the cache directory structure
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

			// Find the latest version (highest semver string)
			var latestVersion string
			for _, version := range versions {
				if !version.IsDir() {
					continue
				}
				if version.Name() > latestVersion {
					latestVersion = version.Name()
				}
			}

			if latestVersion != "" {
				versionPath := filepath.Join(pluginPath, latestVersion)
				skillsDir := filepath.Join(versionPath, "skills")

				pluginSkills, err := loadSkillsFromDirWithVersion(skillsDir, plugin.Name(), latestVersion)
				if err == nil {
					skills = append(skills, pluginSkills...)
				}
			}
		}
	}

	return skills, nil
}

// loadSkillsFromDir loads all skills from a directory (without version)
func loadSkillsFromDir(dir, pluginName string) ([]types.SkillPermissions, error) {
	return loadSkillsFromDirWithVersion(dir, pluginName, "")
}

// loadSkillsFromDirWithVersion loads all skills from a directory with version info
func loadSkillsFromDirWithVersion(dir, pluginName, version string) ([]types.SkillPermissions, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []types.SkillPermissions

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Each skill is in a subdirectory with a SKILL.md file
		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		skill, err := parseSkillFile(skillPath, pluginName, version)
		if err != nil {
			continue
		}

		if len(skill.Permissions) > 0 {
			skills = append(skills, skill)
		}
	}

	return skills, nil
}

// parseSkillFile parses a skill markdown file and extracts frontmatter
func parseSkillFile(path, pluginName, version string) (types.SkillPermissions, error) {
	file, err := os.Open(path)
	if err != nil {
		return types.SkillPermissions{}, err
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
		return types.SkillPermissions{}, nil
	}

	// Parse YAML
	var frontmatter types.SkillFrontmatter
	yamlContent := strings.Join(frontmatterLines, "\n")
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return types.SkillPermissions{}, err
	}

	// Determine skill name
	name := frontmatter.Name
	if name == "" {
		name = filepath.Base(filepath.Dir(path))
	}

	return types.SkillPermissions{
		Name:        name,
		Plugin:      pluginName,
		Version:     version,
		FilePath:    path,
		Permissions: ParsePermissions(parseToolsField(frontmatter.Tools)),
	}, nil
}
