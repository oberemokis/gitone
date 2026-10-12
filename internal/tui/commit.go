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

// typeCommit отдаёт клавиши полю темы: набранное идёт в тему, commit.submit
// коммитит, commit.close закрывает без коммита. Выйти из приложения можно по
// зашитому ctrl+c, стереть символ по зашитому backspace.
func (m Model) typeCommit(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	pressed := key.String()
	switch {
	case pressed == "ctrl+c":
		return m, tea.Quit
	case m.keys.Is(keymap.CommitClose, pressed):
		m.active = noWindow
		m.subject = ""
	case m.keys.Is(keymap.CommitSubmit, pressed):
		subject := strings.TrimSpace(m.subject)
		if subject == "" {
			return m, nil
		}
		m.active = noWindow
		m.subject = ""
		return m, commitCmd(m.dir, subject)
	case pressed == "backspace":
		_, size := utf8.DecodeLastRuneInString(m.subject)
		m.subject = m.subject[:len(m.subject)-size]
	default:
		m.subject += key.Text
	}

	return m, nil
}

// commitBox рисует поле темы. Ширина ограничена экраном, как у окна справки.
func commitBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, subject string, screenWidth int) string {
	width := max(min(60, screenWidth-8), 20)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	title := panel.Bold(true).Foreground(colors.Accent).Render(text.Text("commit.title", nil))
	input := panel.Render("> " + subject + "_")
	// подсказка обрезается: длинная переназначенная клавиша перенесла бы строку
	hintText := windowHint(keys, text, keymap.CommitSubmit, keymap.CommitClose)
	hint := panel.Foreground(colors.TextMuted).Render(ansi.Truncate(hintText, width-2*windowPaddingX, "…"))

	return panel.Padding(1, windowPaddingX).Width(width).Render(strings.Join([]string{title, "", input, "", hint}, "\n"))
}
