// Package files рисует список изменённых файлов. Панель не знает, какой файл
// открыт в диффе, и не получает клавиш: текущий файл передаёт корень, а
// переход по файлам остаётся за его клавишами.
package files

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/tui/theme"
	"gitone/internal/tui/zone"
)

// Отступы внутри панели как у окон: пустая строка сверху, две колонки по
// бокам.
const (
	paddingTop = 1
	paddingX   = 2
)

// View рисует список в прямоугольнике width на height на фоне окон, без рамки:
// от диффа панель отделяет цвет, как окна поверх него. Текущий файл помечен,
// а длинный список прокручивается так, чтобы текущий оставался на экране.
// Пустой список показывает empty: панель без файлов иначе выглядит сломанной.
// Строка файла помечена местом клика zone.Row с его номером (#119).
func View(labels []string, current int, empty string, colors theme.Theme, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	listHeight := max(height-paddingTop, 1)
	start := 0
	if current >= listHeight {
		start = current - listHeight + 1
	}

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	accent := panel.Foreground(colors.Primary)
	pad := strings.Repeat(" ", paddingX)
	row := func(style lipgloss.Style, text string) string {
		if text != "" {
			text = ansi.Truncate(pad+text, max(width-paddingX, 0), "…")
		}
		return style.Render(text + strings.Repeat(" ", max(width-ansi.StringWidth(text), 0)))
	}

	// без места под список отступ не ставится, строка «нет изменений» тоже
	// только в пределах высоты: лишняя строка сдвинула бы подвал
	rows := make([]string, 0, height)
	if height > paddingTop {
		for range paddingTop {
			rows = append(rows, row(panel, ""))
		}
	}
	if len(labels) == 0 && len(rows) < height {
		rows = append(rows, row(panel.Foreground(colors.TextMuted), "  "+empty))
	}
	for index := start; index < len(labels) && len(rows) < height; index++ {
		style, marker := panel, "  "
		if index == current {
			style, marker = accent, "› "
		}
		rows = append(rows, zone.Mark(zone.Row(index), row(style, marker+labels[index])))
	}

	for len(rows) < height {
		rows = append(rows, row(panel, ""))
	}

	return strings.Join(rows, "\n")
}
