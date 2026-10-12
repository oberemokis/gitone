package picker

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	// ширина содержимого окна без отступов, как у окон opencode
	preferredWidth = 56
	paddingX       = 4
	paddingY       = 1
	// заголовок, пустая строка, поиск, пустая строка
	chromeRows  = 4
	maxListRows = 10
	minListRows = 1
)

// View рисует окно не шире width и не выше height: на узком экране оно
// ужимается, а не уходит за край.
func (m Model) View(width, height int) string {
	contentWidth := max(min(preferredWidth, width-2*paddingX), 1)

	// Каждый кусок строки несёт фон панели сам: стиль со сбросом иначе
	// гасит фон до конца строки.
	panel := lipgloss.NewStyle().Background(m.colors.BackgroundPanel)
	dim := panel.Foreground(m.colors.TextMuted)

	// заголовок как у всех окон: клавиша закрытия цветом текста, подпись
	// приглушённая; отключённое закрытие не пишется
	closeHint := ""
	if m.texts.CloseKey != "" {
		closeHint = panel.Render(m.texts.CloseKey) + dim.Render(" "+m.texts.CloseLabel)
	}
	rows := []string{
		justify(panel, contentWidth, panel.Bold(true).Render(m.texts.Title), closeHint),
		justify(panel, contentWidth, "", ""),
		justify(panel, contentWidth, m.search(panel, dim), ""),
		justify(panel, contentWidth, "", ""),
	}

	listRows := max(min(maxListRows, height-chromeRows-2*paddingY), minListRows)
	rows = append(rows, m.list(panel, dim, contentWidth, listRows)...)

	return panel.Padding(paddingY, paddingX).Render(strings.Join(rows, "\n"))
}

func (m Model) search(panel, dim lipgloss.Style) string {
	cursor := panel.Background(m.colors.Primary).Render(" ")
	if m.query == "" {
		return cursor + dim.Render(m.texts.Search)
	}

	return panel.Render(m.query) + cursor
}

// list показывает окно из listRows строк вокруг выделения, чтобы оно не
// уходило за край списка.
func (m Model) list(panel, dim lipgloss.Style, width, listRows int) []string {
	if len(m.matches) == 0 {
		return []string{justify(panel, width, dim.Render(m.texts.NoMatches), "")}
	}

	first := max(min(m.selected-listRows+1, len(m.matches)-listRows), 0)
	first = min(first, m.selected)
	last := min(first+listRows, len(m.matches))

	rows := make([]string, 0, last-first)
	for position := first; position < last; position++ {
		item := m.items[m.matches[position]]

		label, detail := panel, dim
		if position == m.selected {
			label = panel.Background(m.colors.Primary).Foreground(m.colors.BackgroundPanel)
			detail = label
		}

		room := max(width-ansi.StringWidth(item.Detail)-1, 1)
		text := ansi.Truncate(item.Label, room, "…")
		rows = append(rows, justify(label, width, label.Render(text), detail.Render(item.Detail)))
	}

	return rows
}

// justify прижимает left к левому краю, right к правому, промежуток красит
// стилем строки.
func justify(style lipgloss.Style, width int, left, right string) string {
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	return left + style.Render(strings.Repeat(" ", gap)) + right
}
