package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
		return commitBox(m.colors, m.subject, width)
	case logWindow:
		return logBox(m.colors, m.logEntries, m.cursor, width, height)
	case branchWindow:
		return branchBox(m.colors, m.branches, m.cursor, width, height)
	case stashWindow, stashDropWindow:
		return stashBox(m.colors, m.stashes, m.cursor, m.active == stashDropWindow, width, height)
	}

	if m.helpOpen {
		return helpBox(m.colors, width, m.keys.Bindings())
	}
	return ""
}

// navigate разбирает клавиши, общие для списков: выход, закрытие и курсор.
// handled false значит, что клавиша остаётся окну.
func (m Model) navigate(key tea.KeyPressMsg, count int) (next Model, cmd tea.Cmd, handled bool) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit, true
	case "esc":
		m.active = noWindow
		return m, nil, true
	case "down", "j":
		m.cursor = min(m.cursor+1, max(count-1, 0))
		return m, nil, true
	case "up", "k":
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
}

// view рисует список по центру. Строки обрезаются по ширине, а окно
// прокручивается так, чтобы курсор оставался видимым.
func (l list) view(colors theme.Theme, screenWidth, screenHeight int) string {
	width := max(min(l.width, screenWidth-8), 20)
	shown := max(min(len(l.rows), screenHeight-6), 1)
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
		lines = append(lines, panel.Render(ansi.Truncate(marker+l.rows[index], width, "…")))
	}

	last := muted.Render(l.hint)
	if l.question != "" {
		last = panel.Foreground(colors.Error).Render(l.question)
	}
	lines = append(lines, "", last)

	return panel.Padding(1, 2).Width(width).Render(strings.Join(lines, "\n"))
}
