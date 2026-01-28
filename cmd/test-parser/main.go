package main

import (
	"fmt"
	"os"

	"github.com/b-open-io/claude-perms/internal/parser"
)

func main() {
	fmt.Println("Loading permission stats from ~/.claude/projects...")

	stats, err := parser.LoadAllPermissionStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d unique permissions:\n\n", len(stats))

	// Show top 20
	count := 20
	if len(stats) < count {
		count = len(stats)
	}

	fmt.Printf("%-8s %-40s %-12s %s\n", "Count", "Permission", "Last Seen", "Projects")
	fmt.Println("--------", "----------------------------------------", "------------", "--------")

	for i := 0; i < count; i++ {
		s := stats[i]
		lastSeen := s.LastSeen.Format("2006-01-02")
		projCount := len(s.Projects)
		fmt.Printf("%-8d %-40s %-12s %d projects\n", s.Count, s.Permission.Raw, lastSeen, projCount)
	}

	if len(stats) > count {
		fmt.Printf("\n... and %d more\n", len(stats)-count)
	}
}
