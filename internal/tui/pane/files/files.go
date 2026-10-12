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

// View рисует список в прямоугольнике width на height. Справа идёт вертикальная
// черта, отделяющая список от диффа. Текущий файл помечен, а длинный список
// прокручивается так, чтобы текущий оставался на экране.
func View(labels []string, current int, colors theme.Theme, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	contentWidth := max(width-1, 0)
	start := 0
	if current >= height {
		start = current - height + 1
	}

	muted := lipgloss.NewStyle().Foreground(colors.TextMuted)
	accent := lipgloss.NewStyle().Foreground(colors.Primary)

	rows := make([]string, 0, height)
	for index := start; index < len(labels) && len(rows) < height; index++ {
		text, style := "  "+labels[index], lipgloss.NewStyle()
		if index == current {
			text, style = "› "+labels[index], accent
		}

		text = ansi.Truncate(text, contentWidth, "…")
		rows = append(rows, style.Render(text+strings.Repeat(" ", max(contentWidth-ansi.StringWidth(text), 0))))
	}

	for len(rows) < height {
		rows = append(rows, strings.Repeat(" ", contentWidth))
	}

	border := muted.Render("│")
	for index, row := range rows {
		rows[index] = row + border
	}

	return strings.Join(rows, "\n")
}
