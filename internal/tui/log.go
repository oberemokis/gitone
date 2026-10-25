package tui

import (
	"image/color"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// logLimit ограничивает лог: большой репозиторий без предела грузился бы
// секундами ради списка, который всё равно не листают до конца.
const logLimit = 200

// logMsg несёт список коммитов под фильтром или ошибку git. После правки
// истории (#219) курсор открытого лога встаёт на коммит follow (тема и автор:
// хэш после rebase другой), а без него на строку at; -1 значит тот же коммит,
// что был под курсором. request это номер запроса (requestLog).
type logMsg struct {
	repo    git.Repo
	request int
	entries []git.LogEntry
	filter  git.LogFilter
	err     error
	follow  git.LogEntry
	at      int
}

func (msg logMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.logArrived(msg), nil }
func (msg logMsg) from() git.Repo                     { return msg.repo }

// requestLog просит у git лог под фильтром filter. Номер запроса уходит в
// ответ, и лог принимает только ответ на последний: фильтр автора git ищет по
// всей истории секунды, сброс фильтра приходит раньше, и поздний ответ лёг бы
// поверх сброса (#380).
func (m Model) requestLog(filter git.LogFilter) (Model, tea.Cmd) {
	m.logRequest++
	repo, request := m.repo, m.logRequest
	return m, func() tea.Msg {
		entries, err := repo.Log(logLimit, filter)
		return logMsg{repo: repo, request: request, entries: entries, filter: filter, err: err, at: -1}
	}
}

// logArrived открывает окно лога (arrived). Список под новым фильтром или
// после правки истории ложится в открытый лог (#226), и под вопросом над ним
// тоже. У нового файла истории нет: пустое окно сказало бы меньше тоста
// (#241), а в открытой истории файла пустой список это фильтр автора без
// совпадений.
func (m Model) logArrived(msg logMsg) Model {
	switch {
	case msg.request != m.logRequest:
		return m
	case msg.err == nil && msg.filter.File && len(msg.entries) == 0 && m.active == noWindow:
		return m.putNotice(m.text.Text("history.empty", i18n.Args{"path": visible(msg.filter.Path)}))
	case msg.err == nil && m.beneath() == logWindow:
		return m.refilterLog(msg)
	}
	return m.arrived(listArrival{shown: logWindow, err: msg.err, set: func(m *Model) { m.logEntries, m.logFilter = msg.entries, msg.filter }})
}

// fileHistory открывает лог коммитов, которые трогали открытый файл (#241):
// фильтр по файлу из диффа, буквальный путь и история за переименованием
// (`--follow`). Работает на любом диффе, путь у файла тот же.
func (m Model) fileHistory() (tea.Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m, nil
	}

	path := stagedKey(m.files[m.current])
	if path == "" {
		return m.putNotice(m.text.Text("stage.noFile", nil)), nil
	}
	return m.requestLog(git.LogFilter{Path: path, File: true})
}

// refilterLog ставит в открытый лог список под новым фильтром (#226). Курсор
// остаётся на том же коммите, если он есть в новом списке, иначе встаёт на
// первый: так после сброса фильтра не теряется место.
func (m Model) refilterLog(msg logMsg) Model {
	hash := ""
	if shown := m.logShown(); m.cursor < len(shown) {
		hash = shown[m.cursor].Hash
	}

	m.logEntries, m.logFilter = msg.entries, msg.filter
	shown := m.logShown()
	if index := nearestCommit(shown, msg.follow, msg.at); index >= 0 {
		m.cursor = index
		return m
	}
	if msg.at >= 0 {
		m.cursor = commitRow(shown, 0, min(msg.at, len(shown)-1))
		return m
	}
	m.cursor = max(slices.IndexFunc(shown, func(entry git.LogEntry) bool { return entry.Hash == hash && hash != "" }), 0)
	return m
}

// nearestCommit это строка коммита с темой и автором follow, ближайшая к
// строке at: одинаковые темы бывают, а коммит после правки истории стоит
// рядом с прежним местом. -1, если такого коммита нет или follow пуст.
func nearestCommit(entries []git.LogEntry, follow git.LogEntry, at int) int {
	best := -1
	for index, entry := range entries {
		if entry.Hash == "" || follow.Subject == "" || entry.Subject != follow.Subject || entry.Author != follow.Author {
			continue
		}
		if best < 0 || abs(index-at) < abs(best-at) {
			best = index
		}
	}
	return best
}

func abs(number int) int {
	return max(number, -number)
}

// askLogFilter открывает меню фильтров лога (#226), как ctrl+s в lazygit:
// автор и путь задаются отдельно от поиска, поиск остаётся поиском по
// загруженному. Пустое значение снимает свой фильтр.
func (m Model) askLogFilter() Model {
	filter := m.logFilter
	reload := func(m Model, next git.LogFilter) (tea.Model, tea.Cmd) {
		return m.requestLog(next)
	}
	field := func(titleKey, value string, set func(text string) git.LogFilter) menuItem {
		title := m.text.Text(titleKey, nil)
		return menuItem{label: title, run: func(m Model) (tea.Model, tea.Cmd) {
			m = m.openPrompt(title, value, func(m Model, text string) (tea.Model, tea.Cmd) {
				return reload(m, set(text))
			})
			m.prompt.empty = true
			return m, nil
		}}
	}

	return m.openMenu(m.text.Text("logFilter.title", nil), []menuItem{
		field("logFilter.author", filter.Author, func(text string) git.LogFilter {
			filter.Author = text
			return filter
		}),
		field("logFilter.path", filter.Path, func(text string) git.LogFilter {
			filter.Path, filter.File = text, false
			return filter
		}),
		{label: m.text.Text("logFilter.clear", nil), run: func(m Model) (tea.Model, tea.Cmd) {
			return reload(m, git.LogFilter{})
		}},
	})
}

// logTitle это заголовок окна лога с фильтром, если он задан.
func logTitle(text i18n.Dictionary, filter git.LogFilter) string {
	parts := []string{text.Text("log.title", nil)}
	if filter.Author != "" {
		parts = append(parts, text.Text("logFilter.byAuthor", i18n.Args{"author": visible(filter.Author)}))
	}
	if filter.Path != "" {
		parts = append(parts, text.Text("logFilter.byPath", i18n.Args{"path": visible(filter.Path)}))
	}
	return strings.Join(parts, " · ")
}

// logActions это действия окна лога над коммитом под курсором. log.open
// показывает диф коммита в этом же экране, log.revert отменяет коммит новым
// коммитом, правка истории в rewriteAction.
var logActions = []windowAction[git.LogEntry]{
	{action: keymap.LogOpen, run: Model.showCommit},
	{action: keymap.LogCheckout, changesRepo: true, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		return m.changeHistory(m.text.Text("log.detached", i18n.Args{"hash": shortCommit(entry.Hash)}), func(repo git.Repo) error {
			return repo.CheckoutCommit(entry.Hash)
		})
	}},
	{action: keymap.LogNewBranch, changesRepo: true, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		return m.askBranchName(shortCommit(entry.Hash), entry.Hash), nil
	}},
	{action: keymap.LogRevert, changesRepo: true, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		return m.changeHistory(m.text.Text("log.reverted", i18n.Args{"hash": shortCommit(entry.Hash)}), func(repo git.Repo) error {
			return repo.Revert(entry.Hash)
		})
	}},
	{action: keymap.LogReset, changesRepo: true, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		return m.askReset(entry.Hash, shortCommit(entry.Hash), -1), nil
	}},
	{action: keymap.LogSquash, changesRepo: true, run: rewriteAction(git.RewriteSquash)},
	{action: keymap.LogFixup, changesRepo: true, run: rewriteAction(git.RewriteFixup)},
	{action: keymap.LogReword, changesRepo: true, run: rewriteAction(git.RewriteReword)},
	{action: keymap.LogDrop, changesRepo: true, run: rewriteAction(git.RewriteDrop)},
	{action: keymap.LogMoveUp, changesRepo: true, run: rewriteAction(git.RewriteMoveUp)},
	{action: keymap.LogMoveDown, changesRepo: true, run: rewriteAction(git.RewriteMoveDown)},
	{action: keymap.LogFixupCommit, changesRepo: true, run: Model.fixupCommit},
	{action: keymap.LogAutosquash, changesRepo: true, run: Model.autosquash},
	{action: keymap.CherryPickCopy, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		m.copied = entry
		return m.putNotice(m.text.Text("log.copied", i18n.Args{"hash": shortCommit(entry.Hash), "key": firstKey(m.keys, keymap.CherryPickPaste)})), nil
	}},
	{action: keymap.CherryPickPaste, changesRepo: true, run: func(m Model, _ git.LogEntry) (tea.Model, tea.Cmd) {
		return m.pasteCommit()
	}},
	{action: keymap.LogCopy, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		return m.askCopy(entry), nil
	}},
	{action: keymap.CompareWith, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		return m.compareWith(entry.Hash, shortCommit(entry.Hash))
	}},
	{action: keymap.LogBisect, changesRepo: true, run: func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		return m.askBisect(entry.Hash, shortCommit(entry.Hash)), nil
	}},
}

// logHint это действия подсказки окна лога: таблица, поиск и фильтр. Тот же
// список сверяется с группой окна в keymap.
var logHint = actionsOf(logActions, keymap.ListSearch, keymap.LogFilter)

// logKey отдаёт клавиши окну лога: log.filter меню фильтров, остальное
// logActions. Поиск разбирает windowKind.press. Курсор стоит только на
// коммитах.
func (m Model) logKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	shown := m.logShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		next.cursor = commitRow(shown, m.cursor, next.cursor)
		return next, cmd
	}
	// фильтр работает и на пустом списке: иначе его не снять
	if m.keys.Is(keymap.LogFilter, key.String()) {
		return m.askLogFilter(), nil
	}
	return dispatch(m, logActions, key.String(), shown)
}

// showCommit показывает дифф коммита в этом же экране. У первого коммита
// родителя нет, и <коммит>^! дифф не строит.
func (m Model) showCommit(entry git.LogEntry) (tea.Model, tea.Cmd) {
	m.active = noWindow
	if entry.Root {
		return m.putNotice(m.text.Text("log.firstCommitHidden", nil)), nil
	}

	m.args = []string{entry.Hash + "^!"}
	return m.startReload()
}

// pasteCommit переносит скопированный коммит в текущую ветку (#222), как V в
// lazygit. Буфер живёт в памяти до выхода, в git ничего не пишется. Без
// скопированного коммита тост говорит, как его скопировать.
func (m Model) pasteCommit() (tea.Model, tea.Cmd) {
	entry := m.copied
	if entry.Hash == "" {
		return m.putNotice(m.text.Text("log.nothingCopied", i18n.Args{"key": firstKey(m.keys, keymap.CherryPickCopy)})), nil
	}

	return m.changeHistory(m.text.Text("log.picked", i18n.Args{"hash": shortCommit(entry.Hash)}), func(repo git.Repo) error {
		return repo.CherryPick(entry.Hash)
	})
}

// changeHistory закрывает лог и выполняет правку истории change, тост говорит
// итог note. Пока идёт сетевая операция, правка отказывает, окно остаётся.
func (m Model) changeHistory(note string, change func(repo git.Repo) error) (tea.Model, tea.Cmd) {
	if next, refused := m.refuseWhileRemote(); refused {
		return next, nil
	}

	m.active = noWindow
	repo := m.repo
	return m, func() tea.Msg {
		return actionDoneMsg{err: change(repo), note: note}
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
// скрыт: его линии соединяли бы несоседние коммиты (#111). Фильтр по автору и
// пути (#226) стоит в заголовке.
func logBox(p paint, entries []git.LogEntry, filter git.LogFilter, cursor int, query string, typing bool, question string, screenWidth, screenHeight int) string {
	box := list{
		title:    logTitle(p.text, filter),
		close:    p.close(keymap.ListClose),
		cursor:   cursor,
		empty:    []string{p.text.Text("log.empty", nil)},
		hint:     p.hint(logHint...),
		fill:     true,
		question: question,
	}.withSearch(p, query, typing, p.text.Text("log.noMatches", nil))

	panel, muted := p.panel(), p.muted()

	titles := []string{p.text.Text("log.column.commit", nil), p.text.Text("log.column.message", nil), p.text.Text("log.column.author", nil), p.text.Text("log.column.date", nil)}
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

	lanes := []color.Color{p.colors.Primary, p.colors.DiffAdded, p.colors.SyntaxFunction, p.colors.SyntaxString, p.colors.SyntaxType, p.colors.DiffRemoved}
	graph := func(entry git.LogEntry) string {
		if graphWidth == 0 {
			return ""
		}
		return graphCell(panel, lanes, entry.Graph, graphWidth) + panel.Render(strings.Repeat(" ", columnGap))
	}

	box.header = graph(git.LogEntry{}) + tableRow(panel, widths, []lipgloss.Style{muted, muted, muted, muted}, titles...)

	styles := []lipgloss.Style{panel.Foreground(p.colors.Accent), panel, muted, muted}
	box.rows = make([]string, len(entries))
	for index, entry := range entries {
		box.rows[index] = graph(entry)
		if entry.Hash != "" {
			box.rows[index] += tableRow(panel, widths, styles, shortCommit(entry.Hash), entry.Subject, entry.Author, entry.When)
		}
	}

	return box.view(p, screenWidth, screenHeight)
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
		cell = truncate(visible(cell), widths[index], "…")
		row.WriteString(styles[index].Render(cell + strings.Repeat(" ", max(widths[index]-ansi.StringWidth(cell), 0))))
	}
	return row.String()
}
