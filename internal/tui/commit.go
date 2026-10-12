package tui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/git"
	"gitone/internal/tui/theme"
)

// commitCmd записывает индекс с темой. Команда, а не Update: хук может работать
// долго, и экран не должен висеть.
func commitCmd(dir, subject string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.Commit(dir, subject)}
	}
}

// openCommit открывает поле темы. Клавиши уходят в него целиком, как в окно
// поиска.
func (m Model) openCommit() Model {
	m.active = commitWindow
	m.subject = ""
	m.helpOpen = false
	return m
}

// typeCommit отдаёт клавиши полю темы: набранное идёт в тему, enter коммитит,
// esc закрывает без коммита. Выйти из приложения можно по ctrl+c.
func (m Model) typeCommit(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.active = noWindow
		m.subject = ""
	case "enter":
		subject := strings.TrimSpace(m.subject)
		if subject == "" {
			return m, nil
		}
		m.active = noWindow
		m.subject = ""
		return m, commitCmd(m.dir, subject)
	case "backspace":
		_, size := utf8.DecodeLastRuneInString(m.subject)
		m.subject = m.subject[:len(m.subject)-size]
	default:
		m.subject += key.Text
	}

	return m, nil
}

// commitBox рисует поле темы. Ширина ограничена экраном, как у окна справки.
func commitBox(colors theme.Theme, subject string, screenWidth int) string {
	width := max(min(60, screenWidth-8), 20)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	title := panel.Bold(true).Foreground(colors.Accent).Render("Commit")
	input := panel.Render("> " + subject + "_")
	hint := panel.Foreground(colors.TextMuted).Render("enter коммит   esc отмена")

	return panel.Padding(1, 2).Width(width).Render(strings.Join([]string{title, "", input, "", hint}, "\n"))
}
