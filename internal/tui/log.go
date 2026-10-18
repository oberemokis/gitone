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
// экране, log.revert отменяет коммит новым коммитом, list.search открывает
// поле поиска, list.close закрывает окно. Курсор стоит только на коммитах.
func (m Model) logKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.logShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		next.cursor = commitRow(shown, m.cursor, next.cursor)
		return next, cmd
	}
	if len(shown) == 0 {
		return m, nil
	}

	entry, pressed := shown[m.cursor], key.String()
	switch {
	case m.keys.Is(keymap.LogOpen, pressed):
		return m.showCommit(entry)
	case m.keys.Is(keymap.LogRevert, pressed):
		return m.changeHistory(m.text.Text("log.reverted", i18n.Args{"hash": entry.Hash[:7]}), func(dir string) error {
			return git.Revert(dir, entry.Hash)
		})
	case m.keys.Is(keymap.CherryPickCopy, pressed):
		m.copied = entry
		return m.putNotice(m.text.Text("log.copied", i18n.Args{"hash": entry.Hash[:7], "key": firstKey(m.keys, keymap.CherryPickPaste)}), true), nil
	case m.keys.Is(keymap.CherryPickPaste, pressed):
		return m.pasteCommit()
	case m.keys.Is(keymap.LogNewBranch, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.askBranchName(entry.Hash[:7], entry.Hash), nil
	case m.keys.Is(keymap.LogCheckout, pressed):
		return m.changeHistory(m.text.Text("log.detached", i18n.Args{"hash": entry.Hash[:7]}), func(dir string) error {
			return git.CheckoutCommit(dir, entry.Hash)
		})
	case m.keys.Is(keymap.LogCopy, pressed):
		return m.askCopy(entry), nil
	}

	return m, nil
}

// showCommit показывает дифф коммита в этом же экране. У первого коммита
// родителя нет, и <коммит>^! дифф не строит.
func (m Model) showCommit(entry git.LogEntry) (tea.Model, tea.Cmd) {
	m.active = noWindow
	if entry.Root {
		return m.putNotice(m.text.Text("log.firstCommitHidden", nil), true), nil
	}

	m.args = []string{entry.Hash + "^!"}
	return m.startReload()
}

// pasteCommit переносит скопированный коммит в текущую ветку (#222), как V в
// lazygit. Буфер живёт в памяти до выхода, в git ничего не пишется. Без
// скопированного коммита подвал говорит, как его скопировать.
func (m Model) pasteCommit() (tea.Model, tea.Cmd) {
	entry := m.copied
	if entry.Hash == "" {
		return m.putNotice(m.text.Text("log.nothingCopied", i18n.Args{"key": firstKey(m.keys, keymap.CherryPickCopy)}), false), nil
	}

	return m.changeHistory(m.text.Text("log.picked", i18n.Args{"hash": entry.Hash[:7]}), func(dir string) error {
		return git.CherryPick(dir, entry.Hash)
	})
}

// changeHistory закрывает лог и выполняет правку истории change, подвал говорит
// итог note. Пока идёт pull или push, правка отказывает, окно остаётся.
func (m Model) changeHistory(note string, change func(dir string) error) (tea.Model, tea.Cmd) {
	if next, refused := m.refuseWhileRemote(); refused {
		return next, nil
	}

	m.active = noWindow
	dir := m.dir
	return m, func() tea.Msg {
		return actionDoneMsg{err: change(dir), note: note}
	}
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

// logShown это коммиты под фильтром поиска: запрос в начале хэша либо в теме
// или авторе. Хэш сравнивается по префиксу: видно семь символов из сорока, и
// подстрока по всем сорока оставляла бы коммиты, где запрос нигде не виден
// (#153). Полный хэш из буфера начинается с видимых семи и находит коммит.
// Ищется по загруженным коммитам, без вызова git. У строк соединения графа
// текста нет, под фильтром они уходят.
func (m Model) logShown() []git.LogEntry {
	if m.query == "" {
		return m.logEntries
	}

	query := strings.ToLower(m.query)
	var shown []git.LogEntry
	for _, entry := range m.logEntries {
		if strings.HasPrefix(entry.Hash, query) || strings.Contains(strings.ToLower(entry.Subject+"\x00"+entry.Author), query) {
			shown = append(shown, entry)
		}
	}
	return shown
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
		hint:   windowHint(colors, keys, text, keymap.LogOpen, keymap.LogCheckout, keymap.LogNewBranch, keymap.LogRevert, keymap.CherryPickCopy, keymap.CherryPickPaste, keymap.LogCopy, keymap.ListSearch),
		fill:   true,
	}.withSearch(colors, text, keys, query, typing, text.Text("log.noMatches", nil))

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)

	titles := []string{text.Text("log.column.commit", nil), text.Text("log.column.message", nil), text.Text("log.column.author", nil), text.Text("log.column.date", nil)}
	authorWidth, dateWidth := ansi.StringWidth(titles[2]), ansi.StringWidth(titles[3])
	for _, entry := range entries {
		authorWidth = max(authorWidth, ansi.StringWidth(visible(entry.Author)))
		dateWidth = max(dateWidth, ansi.StringWidth(visible(entry.When)))
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
		cell = ansi.Truncate(visible(cell), widths[index], "…")
		row.WriteString(styles[index].Render(cell + strings.Repeat(" ", widths[index]-ansi.StringWidth(cell))))
	}
	return row.String()
}
