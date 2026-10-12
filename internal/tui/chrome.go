package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	footerHeight = 1
	// ponytail: цвет зашит, переедет в textMuted темы вместе с themes/*.json
	mutedColor = "8"
)

func footer(width int) string {
	var hints []string
	for _, binding := range bindings() {
		if binding.label != "" {
			hints = append(hints, displayKey(binding.keys[0])+" "+muted(binding.label))
		}
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, strings.Join(hints, "   "))
}

func muted(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(mutedColor)).Render(text)
}
