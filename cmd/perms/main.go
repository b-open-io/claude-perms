package main

import (
	"fmt"
	"log"
	"os"

	"github.com/b-open-io/claude-perms/internal"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Setup debug logging - write directly to ensure it works
	logFile, err := os.OpenFile("/tmp/perms-debug.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Write directly to file to ensure it works
	logFile.WriteString("=== Starting perms TUI ===\n")
	logFile.Sync()

	log.SetOutput(logFile)
	log.Println("Log initialized")

	p := tea.NewProgram(
		internal.NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	log.Println("Running program...")
	if _, err := p.Run(); err != nil {
		log.Printf("Error running program: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	log.Println("Program exited normally")
}
