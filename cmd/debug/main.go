package main

import (
	"fmt"
	"os"

	"github.com/b-open-io/claude-perms/internal/parser"
)

func main() {
	fmt.Fprintln(os.Stderr, "DEBUG: Starting data load...")

	// Check home dir
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: Error getting home: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "DEBUG: Home dir: %s\n", home)
	}

	// Check projects dir exists
	projectsDir := home + "/.claude/projects"
	if _, err := os.Stat(projectsDir); err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: Projects dir error: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "DEBUG: Projects dir exists\n")
	}

	// Try loading
	stats, err := parser.LoadAllPermissionStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: Load error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Loaded %d permissions\n", len(stats))

	if len(stats) > 0 {
		fmt.Fprintf(os.Stderr, "DEBUG: First permission: %s (count: %d)\n",
			stats[0].Permission.Raw, stats[0].Count)
	}
}
