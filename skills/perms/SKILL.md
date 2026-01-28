---
name: perms
description: Analyze Claude Code permission usage patterns and generate commands to apply frequently-used permissions
---

# Permission Analyzer

Launches a TUI to analyze permission usage across your Claude Code sessions.

## What It Does

- **Frequency Analysis**: Shows which permissions are requested most often
- **Status Tracking**: Indicates which permissions are already approved (user or project level)
- **Apply Commands**: Generates commands to add permissions to your settings

## Usage

Run the TUI:

```bash
~/.claude/plugins/cache/b-open-io/claude-perms/*/bin/perms
```

## Controls

| Key | Action |
|-----|--------|
| `j`/`k` or `↓`/`↑` | Navigate list |
| `Enter` | Open apply modal for selected permission |
| `Tab` | Switch between Frequency/Matrix views |
| `/` | Filter permissions |
| `u` | Copy user-level apply command |
| `p` | Copy project-level apply command |
| `Esc` | Close modal / clear filter |
| `q` | Quit |

## Data Sources

The analyzer reads from:
- Session logs (`~/.claude/projects/*/sessions-index.json`)
- User settings (`~/.claude/settings.local.json`)
- Project settings (`.claude/settings.local.json`)
- Agent declarations from plugins
- Skill declarations from plugins

## Apply Permissions

When you select a permission and press `u` or `p`, the tool copies a command to add that permission to either:

- **User-level** (`~/.claude/settings.local.json`): Applies across all projects
- **Project-level** (`.claude/settings.local.json`): Applies only to current project
