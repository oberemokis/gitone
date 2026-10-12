package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	// ширина содержимого окна без отступов
	helpWidth = 44
	// ponytail: цвета зашиты, переедут в backgroundPanel и accent темы вместе с themes/*.json
	panelColor  = "#141414"
	accentColor = "#9d7cd8"
)

func helpBox() string {
	// Каждый кусок строки несёт фон сам: Lip Gloss закрывает жирный полным
	// сбросом стиля, и без этого фон панели гаснет до конца строки.
	panel := lipgloss.NewStyle().Background(lipgloss.Color(panelColor))
	bold := panel.Bold(true)
	heading := bold.Foreground(lipgloss.Color(accentColor))
	dim := panel.Foreground(lipgloss.Color(mutedColor))

	closeKey := displayKey(keysFor(actionCloseHelp)[0])
	rows := []string{justify(panel, bold.Render("Hotkeys"), dim.Render(closeKey))}

	order, groups := groupByCategory(bindings())
	for _, title := range order {
		rows = append(rows, "", justify(panel, heading.Render(string(title)), ""))

		for _, binding := range groups[title] {
			displayed := make([]string, len(binding.keys))
			for index, key := range binding.keys {
				displayed[index] = displayKey(key)
			}

			rows = append(rows, justify(panel, panel.Render(binding.description), dim.Render(strings.Join(displayed, ", "))))
		}
	}

	return panel.Padding(1, 4).Render(strings.Join(rows, "\n"))
}

// groupByCategory пропускает клавиши без описания: им не место в справке.
func groupByCategory(all []binding) ([]category, map[category][]binding) {
	var order []category
	groups := map[category][]binding{}

	for _, binding := range all {
		if binding.description == "" {
			continue
		}

		if _, seen := groups[binding.category]; !seen {
			order = append(order, binding.category)
		}
		groups[binding.category] = append(groups[binding.category], binding)
	}

	return order, groups
}

func justify(panel lipgloss.Style, left, right string) string {
	gap := max(helpWidth-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + panel.Render(strings.Repeat(" ", gap)) + right
}
