package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/tui"
)

func main() {
	if _, err := tea.NewProgram(tui.New()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
