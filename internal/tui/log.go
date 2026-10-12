package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
)

// logLimit ограничивает лог: большой репозиторий без предела грузился бы
// секундами ради списка, который всё равно не листают до конца.
const logLimit = 200

// logMsg несёт список коммитов или ошибку git.
type logMsg struct {
	entries []git.LogEntry
	err     error
}

func logCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := git.Log(dir, logLimit)
		return logMsg{entries: entries, err: err}
	}
}

// logKey отдаёт клавиши окну лога. log.open показывает диф коммита в этом же
// экране, list.close закрывает окно.
func (m Model) logKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.logEntries)); handled {
		return next, cmd
	}
	if !m.keys.Is(keymap.LogOpen, key.String()) || len(m.logEntries) == 0 {
		return m, nil
	}

	entry := m.logEntries[m.cursor]
	m.active = noWindow
	if entry.Root {
		m.notice = m.text.Text("log.firstCommitHidden", nil)
		return m, nil
	}

	m.args = []string{entry.Hash + "^!"}
	return m.startReload()
}

// Ширина столбцов лога: хэш короткий, автор и дата по самому длинному
// значению, но не шире предела, чтобы теме оставалось место.
const (
	hashColumn      = 7
	maxAuthorColumn = 20
	maxDateColumn   = 16
	columnGap       = 2
)

// logBox рисует лог на весь экран таблицей: хэш, тема, автор, дата, над ними
// заголовки столбцов (#109). Ширина столбцов общая для всех строк, тема
// занимает остаток. Хэш выделен цветом accent, по нему коммит ищут глазами,
// автор и дата приглушённые. Каждый кусок строки несёт фон окна сам:
// вложенный стиль со сбросом иначе погасил бы фон.
func logBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, entries []git.LogEntry, cursor, screenWidth, screenHeight int) string {
	box := list{
		title:  text.Text("log.title", nil),
		close:  closeHint(colors, keys, text, keymap.ListClose),
		cursor: cursor,
		empty:  []string{text.Text("log.empty", nil)},
		hint:   windowHint(colors, keys, text, keymap.LogOpen),
		fill:   true,
	}

	titles := []string{text.Text("log.column.commit", nil), text.Text("log.column.message", nil), text.Text("log.column.author", nil), text.Text("log.column.date", nil)}
	authorWidth, dateWidth := ansi.StringWidth(titles[2]), ansi.StringWidth(titles[3])
	for _, entry := range entries {
		authorWidth = max(authorWidth, ansi.StringWidth(entry.Author))
		dateWidth = max(dateWidth, ansi.StringWidth(entry.When))
	}
	widths := []int{hashColumn, 0, min(authorWidth, maxAuthorColumn), min(dateWidth, maxDateColumn)}
	widths[1] = max(box.rowWidth(screenWidth)-widths[0]-widths[2]-widths[3]-3*columnGap, 1)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)
	box.header = tableRow(panel, widths, muted, muted, muted, muted, titles...)

	box.rows = make([]string, len(entries))
	for index, entry := range entries {
		box.rows[index] = tableRow(panel, widths, panel.Foreground(colors.Accent), panel, muted, muted, entry.Hash[:7], entry.Subject, entry.Author, entry.When)
	}

	return box.view(colors, screenWidth, screenHeight)
}

// tableRow собирает строку таблицы: каждая ячейка своим стилем, обрезана
// «…» и дополнена пробелами до ширины столбца, между столбцами промежуток.
func tableRow(panel lipgloss.Style, widths []int, hash, subject, author, date lipgloss.Style, cells ...string) string {
	styles := []lipgloss.Style{hash, subject, author, date}

	var row strings.Builder
	for index, cell := range cells {
		if index > 0 {
			row.WriteString(panel.Render(strings.Repeat(" ", columnGap)))
		}
		cell = ansi.Truncate(cell, widths[index], "…")
		row.WriteString(styles[index].Render(cell + strings.Repeat(" ", widths[index]-ansi.StringWidth(cell))))
	}
	return row.String()
}
