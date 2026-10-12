package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	// ширина содержимого окна без отступов
	helpWidth = 44
	// ponytail: цвет зашит, переедет в backgroundPanel темы вместе с themes/*.json
	panelColor = "#141414"
)

func helpBox() string {
	// Каждый кусок строки несёт фон сам: Lip Gloss закрывает жирный полным
	// сбросом стиля, и без этого фон панели гаснет до конца строки.
	panel := lipgloss.NewStyle().Background(lipgloss.Color(panelColor))
	bold := panel.Bold(true)
	dim := panel.Foreground(lipgloss.Color(mutedColor))

	closeKey := displayKey(keysFor(actionCloseHelp)[0])
	rows := []string{justify(panel, bold.Render("Hotkeys"), dim.Render(closeKey)), ""}

	for _, binding := range bindings() {
		if binding.description == "" {
			continue
		}

		displayed := make([]string, len(binding.keys))
		for index, key := range binding.keys {
			displayed[index] = displayKey(key)
		}

		rows = append(rows, justify(panel, panel.Render(binding.description), dim.Render(strings.Join(displayed, ", "))))
	}

	return panel.Padding(1, 4).Render(strings.Join(rows, "\n"))
}

func justify(panel lipgloss.Style, left, right string) string {
	gap := max(helpWidth-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + panel.Render(strings.Repeat(" ", gap)) + right
}
