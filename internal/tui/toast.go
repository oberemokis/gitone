package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Заметка, ошибка или итог, стоит тостом справа под заголовком, как в
// opencode (#371): фон панели, рамка ┃ слева и справа цветом вида, по две
// колонки отступа по бокам и строка сверху и снизу. Подвал держит только
// состояние: спиннеры и вопрос с главного экрана.

const (
	// toastWidth это наибольшая ширина тоста вместе с рамкой, как у opencode
	toastWidth = 60
	// toastMargin это отступ тоста от правого края экрана
	toastMargin = 2
	// toastPadding это поле текста у рамки
	toastPadding = 2
)

// toast рисует заметку не шире экрана шириной screenWidth и не выше height
// строк. Каждая строка заметки проходит visible и переносится по словам,
// поэтому многострочная ошибка git видна целиком, пока помещается; что не
// поместилось, обрезается «…» в последней строке. Без заметки или места
// тоста нет.
func (m Model) toast(screenWidth, height int) string {
	width := min(toastWidth, screenWidth-3*toastMargin)
	inner := width - 2 - 2*toastPadding
	if m.notice == "" || inner < 1 || height < 3 {
		return ""
	}

	var lines []string
	for line := range strings.SplitSeq(m.notice, "\n") {
		lines = append(lines, wrapToast(visible(line), inner)...)
	}
	if room := height - 2; len(lines) > room {
		lines = lines[:room]
		lines[room-1] = truncate(lines[room-1], inner-1, "") + "…"
	}

	edge := m.colors.Error
	if m.notice == m.calm {
		edge = m.colors.Primary
	}
	panel := lipgloss.NewStyle().Background(m.colors.BackgroundPanel)
	border := panel.Foreground(edge).Render("┃")
	row := func(text string) string {
		field := strings.Repeat(" ", toastPadding)
		return border + panel.Render(field+text+strings.Repeat(" ", max(inner-ansi.StringWidth(text), 0))+field) + border
	}

	rows := []string{row("")}
	for _, line := range lines {
		rows = append(rows, row(line))
	}
	return strings.Join(append(rows, row("")), "\n")
}

// wrapToast переносит строку заметки по словам в ширину inner. ansi.Wrap
// меряет keycap уже, чем StringWidth, и оставляет строку шире inner: её хвост
// уходит на следующую строку (#377). Символ шире самого тоста (CJK при inner 1)
// не помещается никуда и отбрасывается.
func wrapToast(text string, inner int) []string {
	var lines []string
	for _, line := range strings.Split(ansi.Wrap(text, inner, ""), "\n") {
		for ansi.StringWidth(line) > inner {
			head := truncate(line, inner, "")
			if head == "" {
				head, _ = ansi.FirstGraphemeCluster(line, ansi.GraphemeWidth)
			} else {
				lines = append(lines, head)
			}
			line = line[len(head):]
		}
		lines = append(lines, line)
	}
	return lines
}
