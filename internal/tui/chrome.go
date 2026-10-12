package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"gitone/internal/diff"
	"gitone/internal/tui/theme"
)

const (
	// заголовок и пустая строка под ним
	headerHeight = 2
	footerHeight = 1
)

// header показывает путь открытого файла и счётчики строк, как заголовок
// critique: «examples.ts +6-1».
func header(width int, file diff.File, colors theme.Theme) string {
	title := file.NewPath
	switch {
	case file.Status == diff.StatusRenamed || file.Status == diff.StatusCopied:
		title = file.OldPath + " → " + file.NewPath
	case title == "":
		title = file.OldPath
	}

	if file.Added > 0 || file.Removed > 0 {
		added := lipgloss.NewStyle().Foreground(colors.DiffAdded).Render(fmt.Sprintf("+%d", file.Added))
		removed := lipgloss.NewStyle().Foreground(colors.DiffRemoved).Render(fmt.Sprintf("-%d", file.Removed))
		title += " " + added + removed
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

func footer(width int, colors theme.Theme) string {
	muted := lipgloss.NewStyle().Foreground(colors.TextMuted)

	var hints []string
	for _, binding := range bindings() {
		if binding.label != "" {
			hints = append(hints, displayKey(binding.keys[0])+" "+muted.Render(binding.label))
		}
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, strings.Join(hints, "   "))
}
