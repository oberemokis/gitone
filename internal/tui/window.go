package tui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
	"gitone/internal/tui/zone"
)

// window говорит, кто сейчас получает клавиши. Открыто не больше одного окна:
// список, пришедший при открытом окне, отбрасывается. Справки здесь нет: она
// клавиш не забирает и может лежать под окном, например под вопросом discard.
type window int

const (
	noWindow window = iota
	pickerWindow
	commitWindow
	logWindow
	branchWindow
	worktreeWindow
	stashWindow
	// вопросы y/n: любая клавиша отвечает на них, ctrl c тоже это «нет»
	stashDropWindow
	discardWindow
)

// keyPress отдаёт нажатие открытому окну, а без окна переводит его в действие.
func (m Model) keyPress(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.active {
	case discardWindow:
		return m.answerDiscard(key)
	case stashDropWindow:
		return m.answerStashDrop(key)
	case commitWindow:
		return m.typeCommit(key)
	case logWindow:
		return m.logKey(key)
	case branchWindow:
		return m.branchKey(key)
	case worktreeWindow:
		return m.worktreeKey(key)
	case stashWindow:
		return m.stashKey(key)
	}

	// окна выше подвал не гасят: сообщение, пришедшее при открытом окне,
	// должно пережить работу в нём
	m.notice = ""
	if m.active == pickerWindow {
		return m.pickerKey(key)
	}
	if next, handled := m.helpKey(key.String()); handled {
		return next, nil
	}
	return m.handleAction(m.keys.ActionFor(key.String()))
}

// windowBox это окно, которое лежит поверх диффа, или пустая строка. Вопрос
// discard своего рисунка не имеет: он стоит в подвале, а справка остаётся видна.
func (m Model) windowBox() string {
	width, height := m.width, m.bodyHeight()

	switch m.active {
	case pickerWindow:
		return pickerBox(m.colors, m.text, m.keys, m.picker, len(m.files) == 0, width, height)
	case commitWindow:
		return commitBox(m.colors, m.text, m.keys, m.subject, width)
	case logWindow:
		return logBox(m.colors, m.text, m.keys, m.logShown(), m.cursor, m.query, m.searching, width, height)
	case branchWindow:
		return branchBox(m.colors, m.text, m.keys, m.branchesShown(), m.cursor, m.query, m.searching, width, height)
	case worktreeWindow:
		return worktreeBox(m.colors, m.text, m.keys, m.worktreesShown(), m.cursor, m.query, m.searching, width, height)
	case stashWindow, stashDropWindow:
		return stashBox(m.colors, m.text, m.keys, m.stashes, m.cursor, m.active == stashDropWindow, width, height)
	}

	if m.helpOpen {
		return helpBox(m.colors, m.text, width, height, m.helpTop, m.keys)
	}
	return ""
}

// navigate разбирает клавиши, общие для списков: выход, закрытие и курсор.
// ctrl+c зашит: выход из приложения не должен теряться из-за конфига.
// handled false значит, что клавиша остаётся окну.
func (m Model) navigate(key tea.KeyPressMsg, count int) (next Model, cmd tea.Cmd, handled bool) {
	pressed := key.String()
	switch {
	case pressed == "ctrl+c":
		return m, tea.Quit, true
	case m.keys.Is(keymap.ListClose, pressed):
		m.active = noWindow
		return m, nil, true
	case m.keys.Is(keymap.ListDown, pressed):
		m.cursor = min(m.cursor+1, max(count-1, 0))
		return m, nil, true
	case m.keys.Is(keymap.ListUp, pressed):
		m.cursor = max(m.cursor-1, 0)
		return m, nil, true
	}

	return m, nil, false
}

// searchKey разбирает поиск в списке лога, веток и worktree (#110, #118):
// пока поле открыто, клавиши идут в него, list.search его открывает.
// handled false значит, что клавиша остаётся окну.
func (m Model) searchKey(key tea.KeyPressMsg) (next Model, cmd tea.Cmd, handled bool) {
	if m.searching {
		next, cmd = m.typeSearch(key)
		return next, cmd, true
	}
	if m.keys.Is(keymap.ListSearch, key.String()) {
		m.searching, m.query, m.cursor = true, "", 0
		return m, nil, true
	}

	return m, nil, false
}

// typeSearch отдаёт клавиши полю поиска, как полю темы коммита: набранное
// идёт в запрос, listSearch.done оставляет фильтр, listSearch.cancel снимает
// его. Выйти из приложения можно по зашитому ctrl+c, стереть символ по
// зашитому backspace. Курсор после каждой правки запроса встаёт на первую
// найденную строку.
func (m Model) typeSearch(key tea.KeyPressMsg) (Model, tea.Cmd) {
	pressed := key.String()
	switch {
	case pressed == "ctrl+c":
		return m, tea.Quit
	case m.keys.Is(keymap.ListSearchCancel, pressed):
		m.searching, m.query = false, ""
	case m.keys.Is(keymap.ListSearchDone, pressed):
		m.searching = false
		return m, nil
	case pressed == "backspace":
		_, size := utf8.DecodeLastRuneInString(m.query)
		m.query = m.query[:len(m.query)-size]
	default:
		m.query += key.Text
	}

	m.cursor = 0
	return m, nil
}

// matching оставляет элементы, в тексте которых есть запрос без учёта
// регистра. Без запроса список как есть.
func matching[T any](items []T, query string, text func(T) string) []T {
	if query == "" {
		return items
	}

	query = strings.ToLower(query)
	var shown []T
	for _, item := range items {
		if strings.Contains(strings.ToLower(text(item)), query) {
			shown = append(shown, item)
		}
	}
	return shown
}

// list это окно-список поверх диффа: заголовок с закрытием, пустая строка,
// строки с курсором и последняя строка с подсказкой или вопросом.
type list struct {
	title string
	// close это «esc close» справа от заголовка (closeHint)
	close string
	// наибольшая ширина окна, на узком экране оно ужимается
	width int
	// строки без маркера курсора
	rows   []string
	cursor int
	// что показать вместо пустого списка; пустая строка идёт без стиля
	empty []string
	hint  string
	// вопрос вместо подсказки, цветом ошибки
	question string
	// fill: окно на весь экран с равными полями, а не по ширине строк
	fill bool
	// header это строка заголовков столбцов над строками, пустая без них
	header string
	// search это строка поиска над заголовками столбцов, пустая без поиска
	search string
}

// Поля окна на весь экран около 100 пикселей: ячейка терминала примерно
// вдвое выше, чем шире, поэтому колонок вдвое больше, чем строк. На маленьком
// экране поле не больше восьмой части его стороны.
const (
	fillMarginX = 12
	fillMarginY = 5
)

// withSearch ставит над строками поле поиска, пока оно открыто или фильтр не
// снят: «/ запрос», при наборе с курсором и подсказкой поля. Пустой список под
// фильтром говорит noMatches.
func (l list) withSearch(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, query string, typing bool, noMatches string) list {
	if !typing && query == "" {
		return l
	}

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	l.search = panel.Foreground(colors.TextMuted).Render("/ ") + panel.Render(query)
	if typing {
		l.search += panel.Background(colors.Primary).Render(" ")
		l.hint = windowHint(colors, keys, text, keymap.ListSearchDone, keymap.ListSearchCancel)
	}
	l.empty = []string{noMatches}
	return l
}

// windowWidth это ширина окна на экране шириной screenWidth: на весь экран с
// полями или по l.width, но не шире экрана.
func (l list) windowWidth(screenWidth int) int {
	if l.fill {
		return max(screenWidth-2*min(fillMarginX, screenWidth/8), 20)
	}
	return max(min(l.width, screenWidth-8), 20)
}

// rowWidth это ширина строки без маркера курсора: по ней таблица лога делит
// столбцы.
func (l list) rowWidth(screenWidth int) int {
	return l.windowWidth(screenWidth) - 2*windowPaddingX - ansi.StringWidth("› ")
}

// view рисует список по центру. Строки обрезаются по ширине, а окно
// прокручивается так, чтобы курсор оставался видимым.
func (l list) view(colors theme.Theme, screenWidth, screenHeight int) string {
	width := l.windowWidth(screenWidth)
	// без отступов окна, заголовка с пустой строкой под ним и подсказки с
	// пустой строкой над ней, заголовков столбцов и поиска с пустой строкой
	// под ним
	chrome := 7
	if l.header != "" {
		chrome++
	}
	if l.search != "" {
		chrome += 2
	}
	shown := max(min(len(l.rows), screenHeight-chrome), 1)
	if l.fill {
		shown = max(screenHeight-2*min(fillMarginY, screenHeight/8)-chrome+1, 1)
	}
	// Lip Gloss считает отступы в ширину окна, текст помещается в остаток
	content := width - 2*windowPaddingX
	start := max(0, l.cursor-shown+1)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)

	lines := []string{windowTitle(colors, content, l.title, l.close), ""}
	// поиск отделён пустыми строками сверху и снизу (#133)
	if l.search != "" {
		lines = append(lines, ansi.Truncate(l.search, content, "…"), "")
	}
	if l.header != "" && len(l.rows) > 0 {
		lines = append(lines, panel.Render(ansi.Truncate("  "+l.header, content, "…")))
	}
	if len(l.rows) == 0 {
		for _, line := range l.empty {
			if line != "" {
				line = muted.Render(line)
			}
			lines = append(lines, line)
		}
	}

	// строка дополнена до края окна: клик по ней попадает в любом месте
	// (#119). Добивка несёт фон окна сама: куски строки кончаются сбросом
	// стиля, и пробелы после них были бы без фона (#131)
	for index := start; index < len(l.rows) && index < start+shown; index++ {
		marker := "  "
		if index == l.cursor {
			marker = "› "
		}
		row := panel.Render(ansi.Truncate(marker+l.rows[index], content, "…"))
		row += panel.Render(strings.Repeat(" ", max(content-ansi.StringWidth(row), 0)))
		lines = append(lines, zone.Mark(zone.Row(index), row))
	}

	// окно на весь экран держит высоту и с коротким списком
	for l.fill && len(lines) < shown+chrome-5 {
		lines = append(lines, "")
	}

	// подсказка и вопрос обрезаются, как строки: длинные переназначенные
	// клавиши иначе перенесли бы строку, и окно вылезло бы за экран
	last := ansi.Truncate(l.hint, content, muted.Render("…"))
	if l.question != "" {
		last = panel.Foreground(colors.Error).Render(ansi.Truncate(l.question, content, "…"))
	}
	lines = append(lines, "", last)

	return panel.Padding(1, windowPaddingX).Width(width).Render(strings.Join(lines, "\n"))
}

// windowPaddingX это боковой отступ окон списка и коммита.
const windowPaddingX = 2
