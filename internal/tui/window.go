package tui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/keymap"
	"gitone/internal/tui/editor"
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
	tagWindow
	remoteWindow
	reflogWindow
	submoduleWindow
	journalWindow
	recentWindow
	// поле ввода и меню поверх окна или главного экрана (prompt.go, menu.go)
	promptWindow
	menuWindow
	// вопросы y/n: любая клавиша отвечает на них, ctrl c тоже это «нет».
	// questionWindow общий вопрос (question.go), stashDropWindow старше него
	stashDropWindow
	questionWindow
	// windowEnd не окно, а конец перечисления: до него тест полноты ищет у
	// каждого окна запись в windowKinds
	windowEnd
)

// keyPress отдаёт нажатие открытому окну, а без окна переводит его в действие.
// Нажатие без окна гасит тост. Окна, кроме поиска файлов, его не гасят: тост
// гаснет сам по таймеру (notice.go).
func (m Model) keyPress(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if kind, open := windowKinds[m.active]; open {
		return kind.press(m, key)
	}

	m.notice = ""
	// ctrl+c зашит и здесь, как в окнах: app.quit можно отключить, а выход
	// из приложения не должен теряться из-за конфига
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if next, handled := m.helpKey(key.String()); handled {
		return next, nil
	}
	// своя команда стоит на клавише, свободной от действий (#246)
	if command, found := m.keys.CommandFor(key.String()); found {
		return m.runCommand(command)
	}
	return m.handleAction(m.keys.ActionFor(key.String()))
}

// windowBox это окно, которое лежит поверх диффа, или пустая строка. Вопрос
// discard своего рисунка не имеет: он стоит в подвале, а справка остаётся видна.
// Общий вопрос рисуется последней строкой окна, над которым задан (asking).
// modal говорит, что это окно, а не справка: под окном экран гаснет, и клик
// мимо него окно закрывает (#367).
func (m Model) windowBox() (box string, modal bool) {
	shown, question := m.asking()
	if kind := windowKinds[shown]; kind.box != nil {
		return kind.box(m, m.paint(), question), true
	}

	if m.helpOpen {
		return helpBox(m.paint(), m.width, m.bodyHeight(), m.helpTop), false
	}
	return "", false
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
		return m.moveCursor(1, count), nil, true
	case m.keys.Is(keymap.ListUp, pressed):
		return m.moveCursor(-1, count), nil, true
	}

	return m, nil, false
}

// listArrival это ответ git со списком для окна shown (#354).
type listArrival struct {
	shown window
	err   error
	// edit это ответ правки (удаление, переименование): окно он не открывает
	edit bool
	// set кладёт список в модель, start это курсор при открытии окна
	set   func(*Model)
	start int
}

// arrived делает с ответом со списком то, что общее у всех окон-списков.
// Ошибка git уходит в тост при любом окне. Открытое окно или окно под
// вопросом получает новый список, курсор не уходит за край. Список,
// пришедший, когда открыто другое окно, отбрасывается: иначе на экране одно
// окно, а клавиши уходят другому. Ответ правки окна не открывает: если его
// закрыли, остаются итог и перечитывание диффа, их делает окно.
func (m Model) arrived(reply listArrival) Model {
	switch {
	case reply.err != nil:
		return m.say(m.text.Error(reply.err))
	case m.beneath() == reply.shown:
		reply.set(&m)
		return m.moveCursor(0, windowKinds[reply.shown].rows(m))
	case reply.edit || m.active != noWindow:
		return m
	}

	reply.set(&m)
	m.cursor, m.active = reply.start, reply.shown
	m.query, m.searching = "", false
	return m
}

// moveCursor сдвигает курсор на delta строк списка из count строк, не дальше
// первой и последней.
func (m Model) moveCursor(delta, count int) Model {
	m.cursor = max(min(m.cursor+delta, count-1), 0)
	return m
}

// wheelList двигает курсор открытого списка колесом: щелчок это wheelStep
// нажатий list.down или list.up подряд. Вбок списку двигаться некуда. Окно без
// списка (вопросы, поле ввода) колесо пропускает. Окно коммита до сюда не
// доходит: его колесо прокручивает описание (wheelCommit). Список
// прокручивается при рисовании за курсором, поле поиска колесу не мешает:
// курсор ходит по найденному.
func (m Model) wheelList(button tea.MouseButton) Model {
	switch button {
	case tea.MouseWheelDown:
		return windowKinds[m.active].moveBy(m, wheelStep)
	case tea.MouseWheelUp:
		return windowKinds[m.active].moveBy(m, -wheelStep)
	}
	return m
}

// searchKey разбирает поиск в списке окна с windowKind.search (#110, #118):
// пока поле открыто, клавиши идут в него, list.search его открывает. handled
// false значит, что клавиша остаётся окну.
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

// paste кладёт вставку из буфера в открытое текстовое поле: тему и описание
// коммита, поиск файлов или поиск в списке. В описание идёт вся вставка, в
// остальные поля первая строка: перевод строки сработал бы как enter и отправил
// коммит. Без открытого поля вставка ничего не делает (#153).
func (m Model) paste(content string) Model {
	line := pastedLine(content)

	switch {
	case m.active == commitWindow && m.commitField == descriptionField:
		m.description = m.description.Type(content)
	case m.active == commitWindow:
		m.subject += line
	case m.active == pickerWindow:
		m.picker = m.picker.Type(line)
	case m.active == promptWindow:
		m.prompt.text += line
	case m.searching && windowKinds[m.active].search:
		m.query += line
		m.cursor = 0
	}
	return m
}

// pastedLine готовит вставку для однострочного поля так же, как поле описания
// (editor.Clean): ESC из буфера ушёл бы в тему коммита кодом терминала. Из
// вставки остаётся строка до первого перевода строки.
func pastedLine(content string) string {
	line, _, _ := strings.Cut(editor.Clean(content), "\n")
	return line
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
// запросом говорит noMatches. Открытое поле с пустым запросом фильтром не
// считается, там остаётся текст пустого списка (#153).
func (l list) withSearch(p paint, query string, typing bool, noMatches string) list {
	if !typing && query == "" {
		return l
	}

	panel := p.panel()
	l.search = p.muted().Render("/ ") + panel.Render(query)
	if typing {
		l.search += panel.Background(p.colors.Primary).Render(" ")
		l.hint = p.hint(keymap.ListSearchDone, keymap.ListSearchCancel)
	}
	if query != "" {
		l.empty = []string{noMatches}
	}
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
func (l list) view(p paint, screenWidth, screenHeight int) string {
	width := l.windowWidth(screenWidth)
	// Lip Gloss считает отступы в ширину окна, текст помещается в остаток
	content := width - 2*windowPaddingX

	panel, muted := p.panel(), p.muted()

	// вопрос обрезается, как строки: иначе окно вылезло бы за экран. Подсказка
	// переносится по парам «клавиша подпись», когда действий больше, чем
	// помещается в строку (#229)
	bottom := packHint(panel, muted, l.hint, content)
	if l.question != "" {
		bottom = []string{panel.Foreground(p.colors.Error).Render(truncate(l.question, content, "…"))}
	}

	// без двух отступов окна, заголовка с пустой строкой под ним, пустой строки
	// над подсказкой и самой подсказки, заголовков столбцов (одна) и поиска с
	// пустой строкой под ним (две) (#153)
	chrome := 5 + len(bottom)
	if l.header != "" {
		chrome++
	}
	if l.search != "" {
		chrome += 2
	}
	shown := max(min(len(l.rows), screenHeight-chrome), 1)
	if l.fill {
		shown = max(screenHeight-2*min(fillMarginY, screenHeight/8)-chrome, 1)
	}
	start := max(0, l.cursor-shown+1)

	lines := []string{p.title(content, l.title, l.close), ""}
	// поиск отделён пустыми строками сверху и снизу (#133)
	if l.search != "" {
		lines = append(lines, truncate(l.search, content, "…"), "")
	}
	if l.header != "" && len(l.rows) > 0 {
		lines = append(lines, panel.Render(truncate("  "+l.header, content, "…")))
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
		row := panel.Render(truncate(marker+l.rows[index], content, "…"))
		row += panel.Render(strings.Repeat(" ", max(content-ansi.StringWidth(row), 0)))
		lines = append(lines, zone.Mark(zone.Row(index), row))
	}

	// окно на весь экран держит высоту и с коротким списком
	for l.fill && len(lines) < shown+chrome-3-len(bottom) {
		lines = append(lines, "")
	}

	lines = append(append(lines, ""), bottom...)

	return panel.Padding(1, windowPaddingX).Width(width).Render(strings.Join(lines, "\n"))
}

// packHint раскладывает подсказку окна по строкам шириной width: пары
// «клавиша подпись» (windowHint) не рвутся, строка переносится между ними. Пара
// шире строки обрезается «…»: длинную переназначенную клавишу иначе нечем
// показать.
func packHint(panel, muted lipgloss.Style, hint string, width int) []string {
	gap := panel.Render("   ")
	ellipsis := muted.Render("…")

	var lines []string
	line := ""
	for _, pair := range strings.Split(hint, gap) {
		switch {
		case line == "":
			line = pair
		case ansi.StringWidth(line+gap+pair) <= width:
			line += gap + pair
		default:
			lines = append(lines, truncate(line, width, ellipsis))
			line = pair
		}
	}
	return append(lines, truncate(line, width, ellipsis))
}

// windowPaddingX это боковой отступ окон списка и коммита.
const windowPaddingX = 2
