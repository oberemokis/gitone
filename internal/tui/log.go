package tui

import (
	"image/color"
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
// экране, list.search открывает поле поиска, list.close закрывает окно.
func (m Model) logKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.logShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		next.cursor = commitRow(shown, m.cursor, next.cursor)
		return next, cmd
	}
	if !m.keys.Is(keymap.LogOpen, key.String()) || len(shown) == 0 {
		return m, nil
	}

	entry := shown[m.cursor]
	m.active = noWindow
	if entry.Root {
		m.notice = m.text.Text("log.firstCommitHidden", nil)
		return m, nil
	}

	m.args = []string{entry.Hash + "^!"}
	return m.startReload()
}

// commitRow уводит курсор, пришедший из from на строку соединения графа to,
// дальше в ту же сторону до коммита: курсор стоит только на коммитах. Коммита
// в той стороне нет, курсор остаётся в from.
func commitRow(entries []git.LogEntry, from, to int) int {
	step := 1
	if to < from {
		step = -1
	}
	for index := to; index >= 0 && index < len(entries); index += step {
		if entries[index].Hash != "" {
			return index
		}
	}
	return from
}

// logShown это коммиты под фильтром поиска: запрос в хэше, теме или авторе.
// Ищется по загруженным коммитам, без вызова git. У строк соединения графа
// текста нет, под фильтром они уходят.
func (m Model) logShown() []git.LogEntry {
	return matching(m.logEntries, m.query, func(entry git.LogEntry) string {
		return entry.Hash + "\x00" + entry.Subject + "\x00" + entry.Author
	})
}

// Ширина столбцов лога: граф и хэш короткие, автор и дата по самому длинному
// значению, но не шире предела, чтобы теме оставалось место. Граф многих
// параллельных веток обрезается справа.
const (
	maxGraphColumn  = 24
	hashColumn      = 7
	maxAuthorColumn = 20
	maxDateColumn   = 16
	columnGap       = 2
)

// logBox рисует лог на весь экран таблицей: граф, хэш, тема, автор, дата, над
// ними заголовки столбцов (#109). Ширина столбцов общая для всех строк, тема
// занимает остаток. Хэш выделен цветом accent, по нему коммит ищут глазами,
// автор и дата приглушённые. Каждый кусок строки несёт фон окна сам:
// вложенный стиль со сбросом иначе погасил бы фон. Строка поиска стоит над
// таблицей, пока поле открыто или фильтр не снят (#110). Под фильтром граф
// скрыт: его линии соединяли бы несоседние коммиты (#111).
func logBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, entries []git.LogEntry, cursor int, query string, typing bool, screenWidth, screenHeight int) string {
	box := list{
		title:  text.Text("log.title", nil),
		close:  closeHint(colors, keys, text, keymap.ListClose),
		cursor: cursor,
		empty:  []string{text.Text("log.empty", nil)},
		hint:   windowHint(colors, keys, text, keymap.LogOpen, keymap.ListSearch),
		fill:   true,
	}.withSearch(colors, text, keys, query, typing, text.Text("log.noMatches", nil))

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)

	titles := []string{text.Text("log.column.commit", nil), text.Text("log.column.message", nil), text.Text("log.column.author", nil), text.Text("log.column.date", nil)}
	authorWidth, dateWidth := ansi.StringWidth(titles[2]), ansi.StringWidth(titles[3])
	for _, entry := range entries {
		authorWidth = max(authorWidth, ansi.StringWidth(entry.Author))
		dateWidth = max(dateWidth, ansi.StringWidth(entry.When))
	}
	graphWidth := 0
	if query == "" {
		for _, entry := range entries {
			graphWidth = max(graphWidth, len(entry.Graph))
		}
		graphWidth = min(graphWidth, maxGraphColumn)
	}
	// столбец графа вместе с промежутком после него, без графа ноль
	graphColumn := 0
	if graphWidth > 0 {
		graphColumn = graphWidth + columnGap
	}
	widths := []int{hashColumn, 0, min(authorWidth, maxAuthorColumn), min(dateWidth, maxDateColumn)}
	widths[1] = max(box.rowWidth(screenWidth)-graphColumn-widths[0]-widths[2]-widths[3]-3*columnGap, 1)

	lanes := []color.Color{colors.Primary, colors.DiffAdded, colors.SyntaxFunction, colors.SyntaxString, colors.SyntaxType, colors.DiffRemoved}
	graph := func(entry git.LogEntry) string {
		if graphWidth == 0 {
			return ""
		}
		return graphCell(panel, lanes, entry.Graph, graphWidth) + panel.Render(strings.Repeat(" ", columnGap))
	}

	box.header = graph(git.LogEntry{}) + tableRow(panel, widths, []lipgloss.Style{muted, muted, muted, muted}, titles...)

	styles := []lipgloss.Style{panel.Foreground(colors.Accent), panel, muted, muted}
	box.rows = make([]string, len(entries))
	for index, entry := range entries {
		box.rows[index] = graph(entry)
		if entry.Hash != "" {
			box.rows[index] += tableRow(panel, widths, styles, entry.Hash[:7], entry.Subject, entry.Author, entry.When)
		}
	}

	return box.view(colors, screenWidth, screenHeight)
}

// graphCell рисует граф git log --graph шириной width, каждый символ цветом
// своей дорожки по кругу из lanes. Дорожка занимает две колонки, «\» и «/» в
// нечётной колонке относятся к дорожке справа: туда ведёт ответвление и оттуда
// приходит слияние. Рисунок git состоит из ASCII, поэтому байт это колонка.
func graphCell(panel lipgloss.Style, lanes []color.Color, graph string, width int) string {
	var cell strings.Builder
	for column := range width {
		if column >= len(graph) || graph[column] == ' ' {
			cell.WriteString(panel.Render(" "))
			continue
		}
		lane := (column + 1) / 2
		cell.WriteString(panel.Foreground(lanes[lane%len(lanes)]).Render(graph[column : column+1]))
	}
	return cell.String()
}

// tableRow собирает строку таблицы лога и окна веток: каждая ячейка своим
// стилем из styles, обрезана «…» и дополнена пробелами до ширины столбца,
// между столбцами промежуток.
func tableRow(panel lipgloss.Style, widths []int, styles []lipgloss.Style, cells ...string) string {
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
