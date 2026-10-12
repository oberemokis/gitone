package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
	"gitone/internal/tui/zone"
)

const (
	// ширина содержимого окна без отступов
	helpWidth    = 44
	helpPaddingX = 4
)

// helpBox не шире экрана: на узком окно ужимается, а длинные описания
// обрезаются, иначе окно уходило за край и теряло левые колонки. Клик по
// строке закрывает справку и делает действие строки, как меню клавиш lazygit
// (#119).
func helpBox(colors theme.Theme, text i18n.Dictionary, screenWidth int, keys keymap.Keymap) string {
	width := max(min(helpWidth, screenWidth-2*helpPaddingX), 1)

	// Каждый кусок строки несёт фон сам: Lip Gloss закрывает жирный полным
	// сбросом стиля, и без этого фон панели гаснет до конца строки.
	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	heading := panel.Bold(true).Foreground(colors.Accent)
	dim := panel.Foreground(colors.TextMuted)

	// заголовок как у всех окон; клавиши цветом текста, подписи
	// приглушённые, как в подвале (#89)
	rows := []string{windowTitle(colors, width, text.Text("help.title", nil), closeHint(colors, keys, text, keymap.HelpClose))}

	order, groups := groupByCategory(keys.Bindings())
	for _, category := range order {
		rows = append(rows, "", justify(panel, width, heading.Render(text.Text(categoryKey(category), nil)), ""))

		for _, binding := range groups[category] {
			displayed := make([]string, len(binding.Keys))
			for index, key := range binding.Keys {
				displayed[index] = displayKey(key)
			}

			description := text.Text(helpKey(binding.Action), nil)
			row := justify(panel, width, dim.Render(description), panel.Render(strings.Join(displayed, ", ")))
			rows = append(rows, zone.Mark(helpTarget+string(binding.Action), row))
		}
	}

	return panel.Padding(1, helpPaddingX).Render(strings.Join(rows, "\n"))
}

// groupByCategory пропускает действия не для справки и без клавиш: им не
// место в справке.
func groupByCategory(all []keymap.Binding) ([]keymap.Category, map[keymap.Category][]keymap.Binding) {
	var order []keymap.Category
	groups := map[keymap.Category][]keymap.Binding{}

	for _, binding := range all {
		if !binding.Help || len(binding.Keys) == 0 {
			continue
		}

		if _, seen := groups[binding.Category]; !seen {
			order = append(order, binding.Category)
		}
		groups[binding.Category] = append(groups[binding.Category], binding)
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
