package tui

import (
	"strings"
	"unicode/utf8"

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
// экране, log.search открывает поле поиска, list.close закрывает окно и
// снимает фильтр. Пока поле открыто, клавиши идут в него.
func (m Model) logKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.logSearching {
		return m.typeLogSearch(key)
	}
	if m.keys.Is(keymap.LogSearch, key.String()) {
		m.logSearching, m.logQuery, m.cursor = true, "", 0
		return m, nil
	}

	shown := m.logShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		if next.active == noWindow {
			next.logQuery = ""
		}
		return next, cmd
	}
	if !m.keys.Is(keymap.LogOpen, key.String()) || len(shown) == 0 {
		return m, nil
	}

	entry := shown[m.cursor]
	m.active, m.logQuery = noWindow, ""
	if entry.Root {
		m.notice = m.text.Text("log.firstCommitHidden", nil)
		return m, nil
	}

	m.args = []string{entry.Hash + "^!"}
	return m.startReload()
}

// typeLogSearch отдаёт клавиши полю поиска, как полю темы коммита: набранное
// идёт в запрос, logSearch.done оставляет фильтр, logSearch.cancel снимает
// его. Выйти из приложения можно по зашитому ctrl+c, стереть символ по
// зашитому backspace. Курсор после каждой правки запроса встаёт на первую
// найденную строку.
func (m Model) typeLogSearch(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	pressed := key.String()
	switch {
	case pressed == "ctrl+c":
		return m, tea.Quit
	case m.keys.Is(keymap.LogSearchCancel, pressed):
		m.logSearching, m.logQuery = false, ""
	case m.keys.Is(keymap.LogSearchDone, pressed):
		m.logSearching = false
		return m, nil
	case pressed == "backspace":
		_, size := utf8.DecodeLastRuneInString(m.logQuery)
		m.logQuery = m.logQuery[:len(m.logQuery)-size]
	default:
		m.logQuery += key.Text
	}

	m.cursor = 0
	return m, nil
}

// logShown это коммиты под фильтром поиска: подстрока без учёта регистра в
// хэше, теме или авторе. Ищется по загруженным коммитам, без вызова git.
func (m Model) logShown() []git.LogEntry {
	if m.logQuery == "" {
		return m.logEntries
	}

	query := strings.ToLower(m.logQuery)
	var shown []git.LogEntry
	for _, entry := range m.logEntries {
		text := strings.ToLower(entry.Hash + "\x00" + entry.Subject + "\x00" + entry.Author)
		if strings.Contains(text, query) {
			shown = append(shown, entry)
		}
	}
	return shown
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
// вложенный стиль со сбросом иначе погасил бы фон. Строка поиска стоит над
// таблицей, пока поле открыто или фильтр не снят (#110).
func logBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, entries []git.LogEntry, cursor int, query string, typing bool, screenWidth, screenHeight int) string {
	box := list{
		title:  text.Text("log.title", nil),
		close:  closeHint(colors, keys, text, keymap.ListClose),
		cursor: cursor,
		empty:  []string{text.Text("log.empty", nil)},
		hint:   windowHint(colors, keys, text, keymap.LogOpen, keymap.LogSearch),
		fill:   true,
	}

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)
	if typing || query != "" {
		box.search = muted.Render("/ ") + panel.Render(query)
		if typing {
			box.search += panel.Background(colors.Primary).Render(" ")
			box.hint = windowHint(colors, keys, text, keymap.LogSearchDone, keymap.LogSearchCancel)
		}
		box.empty = []string{text.Text("log.noMatches", nil)}
	}

	titles := []string{text.Text("log.column.commit", nil), text.Text("log.column.message", nil), text.Text("log.column.author", nil), text.Text("log.column.date", nil)}
	authorWidth, dateWidth := ansi.StringWidth(titles[2]), ansi.StringWidth(titles[3])
	for _, entry := range entries {
		authorWidth = max(authorWidth, ansi.StringWidth(entry.Author))
		dateWidth = max(dateWidth, ansi.StringWidth(entry.When))
	}
	widths := []int{hashColumn, 0, min(authorWidth, maxAuthorColumn), min(dateWidth, maxDateColumn)}
	widths[1] = max(box.rowWidth(screenWidth)-widths[0]-widths[2]-widths[3]-3*columnGap, 1)

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
