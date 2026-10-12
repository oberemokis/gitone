package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/tui/theme"
)

const (
	// ширина содержимого окна без отступов
	helpWidth    = 44
	helpPaddingX = 4
)

// helpBox не шире экрана: на узком окно ужимается, а длинные описания
// обрезаются, иначе окно уходило за край и теряло левые колонки.
func helpBox(colors theme.Theme, screenWidth int) string {
	width := max(min(helpWidth, screenWidth-2*helpPaddingX), 1)

	// Каждый кусок строки несёт фон сам: Lip Gloss закрывает жирный полным
	// сбросом стиля, и без этого фон панели гаснет до конца строки.
	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	bold := panel.Bold(true)
	heading := bold.Foreground(colors.Accent)
	dim := panel.Foreground(colors.TextMuted)

	closeKey := displayKey(keysFor(actionCloseHelp)[0])
	rows := []string{justify(panel, width, bold.Render("Hotkeys"), dim.Render(closeKey))}

	order, groups := groupByCategory(bindings())
	for _, title := range order {
		rows = append(rows, "", justify(panel, width, heading.Render(string(title)), ""))

		for _, binding := range groups[title] {
			displayed := make([]string, len(binding.keys))
			for index, key := range binding.keys {
				displayed[index] = displayKey(key)
			}

			rows = append(rows, justify(panel, width, panel.Render(binding.description), dim.Render(strings.Join(displayed, ", "))))
		}
	}

	return panel.Padding(1, helpPaddingX).Render(strings.Join(rows, "\n"))
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

// justify прижимает left к левому краю, right к правому. Не влезает, left
// обрезается: клавиши справа важнее хвоста описания.
func justify(panel lipgloss.Style, width int, left, right string) string {
	left = ansi.Truncate(left, max(width-lipgloss.Width(right)-1, 0), "…")
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + panel.Render(strings.Repeat(" ", gap)) + right
}
