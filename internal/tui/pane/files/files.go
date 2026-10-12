// Package files рисует список изменённых файлов. Панель не знает, какой файл
// открыт в диффе, и не получает клавиш: текущий файл передаёт корень, а
// переход по файлам остаётся за его клавишами.
package files

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/tui/theme"
)

// View рисует список в прямоугольнике width на height на фоне окон, без рамки:
// от диффа панель отделяет цвет, как окна поверх него. Текущий файл помечен,
// а длинный список прокручивается так, чтобы текущий оставался на экране.
// Пустой список показывает empty: панель без файлов иначе выглядит сломанной.
func View(labels []string, current int, empty string, colors theme.Theme, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	start := 0
	if current >= height {
		start = current - height + 1
	}

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	accent := panel.Foreground(colors.Primary)
	// последняя колонка пустая, чтобы имя не упиралось в дифф
	row := func(style lipgloss.Style, text string) string {
		text = ansi.Truncate(text, max(width-1, 0), "…")
		return style.Render(text + strings.Repeat(" ", max(width-ansi.StringWidth(text), 0)))
	}

	rows := make([]string, 0, height)
	if len(labels) == 0 {
		rows = append(rows, row(panel.Foreground(colors.TextMuted), "  "+empty))
	}
	for index := start; index < len(labels) && len(rows) < height; index++ {
		if index == current {
			rows = append(rows, row(accent, "› "+labels[index]))
			continue
		}
		rows = append(rows, row(panel, "  "+labels[index]))
	}

	for len(rows) < height {
		rows = append(rows, row(panel, ""))
	}

	return strings.Join(rows, "\n")
}
