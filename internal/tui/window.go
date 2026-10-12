package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
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
	return m.handleAction(m.keys.ActionFor(key.String()))
}

// windowBox это окно, которое лежит поверх диффа, или пустая строка. Вопрос
// discard своего рисунка не имеет: он стоит в подвале, а справка остаётся видна.
func (m Model) windowBox() string {
	width, height := m.width, m.bodyHeight()

	switch m.active {
	case pickerWindow:
		return m.picker.View(width, height)
	case commitWindow:
		return commitBox(m.colors, m.text, m.keys, m.subject, width)
	case logWindow:
		return logBox(m.colors, m.text, m.keys, m.logEntries, m.cursor, width, height)
	case branchWindow:
		return branchBox(m.colors, m.text, m.keys, m.branches, m.cursor, width, height)
	case worktreeWindow:
		return worktreeBox(m.colors, m.text, m.keys, m.worktrees, m.cursor, width, height)
	case stashWindow, stashDropWindow:
		return stashBox(m.colors, m.text, m.keys, m.stashes, m.cursor, m.active == stashDropWindow, width, height)
	}

	if m.helpOpen {
		return helpBox(m.colors, m.text, width, m.keys)
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

// list это окно-список поверх диффа: заголовок, строки с курсором и последняя
// строка с подсказкой или вопросом.
type list struct {
	title string
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
}

// Поля окна на весь экран около 100 пикселей: ячейка терминала примерно
// вдвое выше, чем шире, поэтому колонок вдвое больше, чем строк. На маленьком
// экране поле не больше восьмой части его стороны.
const (
	fillMarginX = 12
	fillMarginY = 5
)

// view рисует список по центру. Строки обрезаются по ширине, а окно
// прокручивается так, чтобы курсор оставался видимым.
func (l list) view(colors theme.Theme, screenWidth, screenHeight int) string {
	width := max(min(l.width, screenWidth-8), 20)
	shown := max(min(len(l.rows), screenHeight-6), 1)
	if l.fill {
		width = max(screenWidth-2*min(fillMarginX, screenWidth/8), 20)
		// без отступов окна, заголовка и подсказки с пустой строкой над ней
		shown = max(screenHeight-2*min(fillMarginY, screenHeight/8)-5, 1)
	}
	// Lip Gloss считает отступы в ширину окна, текст помещается в остаток
	content := width - 2*windowPaddingX
	start := max(0, l.cursor-shown+1)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)

	lines := []string{panel.Bold(true).Foreground(colors.Accent).Render(l.title)}
	if len(l.rows) == 0 {
		for _, line := range l.empty {
			if line != "" {
				line = muted.Render(line)
			}
			lines = append(lines, line)
		}
	}

	for index := start; index < len(l.rows) && index < start+shown; index++ {
		marker := "  "
		if index == l.cursor {
			marker = "› "
		}
		lines = append(lines, panel.Render(ansi.Truncate(marker+l.rows[index], content, "…")))
	}

	// окно на весь экран держит высоту и с коротким списком
	for l.fill && len(lines) < shown+1 {
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
