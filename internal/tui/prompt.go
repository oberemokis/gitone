package tui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/keymap"
)

// prompt это однострочное поле ввода поверх диффа (#227): имя новой ветки,
// тега, сообщение stash. Что делать с набранным, решает done, его задаёт тот,
// кто открыл поле. back это окно, в которое поле возвращает по prompt.close и
// перед done: из окна веток поле открывается поверх списка и уходит обратно к
// нему. Пустой ввод done не получает, пока не разрешён empty. secret рисует
// набранное точками, credential отмечает поле с вопросом git о логине и пароле
// (credential.go).
type prompt struct {
	title      string
	text       string
	empty      bool
	secret     bool
	credential bool
	done       func(m Model, text string) (tea.Model, tea.Cmd)
	back       window
}

// openPrompt открывает поле с заголовком title и начальным текстом text.
func (m Model) openPrompt(title, text string, done func(m Model, text string) (tea.Model, tea.Cmd)) Model {
	m.prompt = prompt{title: title, text: text, done: done, back: m.active}
	m.active = promptWindow
	m.helpOpen = false
	return m
}

// typePrompt отдаёт клавиши полю: prompt.submit отдаёт набранное без пробелов
// по краям, prompt.close возвращает в прежнее окно. Выйти из приложения можно по
// зашитому ctrl+c, стереть символ по зашитому backspace.
func (m Model) typePrompt(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	pressed := key.String()
	switch {
	case pressed == "ctrl+c":
		return m, tea.Quit
	case m.keys.Is(keymap.PromptClose, pressed):
		m.active = m.prompt.back
	case m.keys.Is(keymap.PromptSubmit, pressed):
		text := strings.TrimSpace(m.prompt.text)
		if text == "" && !m.prompt.empty {
			return m, nil
		}
		m.active = m.prompt.back
		return m.prompt.done(m, text)
	case pressed == "backspace":
		_, size := utf8.DecodeLastRuneInString(m.prompt.text)
		m.prompt.text = m.prompt.text[:len(m.prompt.text)-size]
	default:
		m.prompt.text += key.Text
	}

	return m, nil
}

// promptBox рисует поле, как тему в окне коммита: заголовок с закрытием, строку
// «> текст_» и подсказку. Длинный текст не переносится, виден его конец, где
// идёт набор.
func promptBox(p paint, field prompt, screenWidth int) string {
	width := commitWidth(screenWidth)
	content := width - 2*windowPaddingX

	panel := p.panel()
	title := p.title(content, field.title, p.close(keymap.PromptClose))

	typed := visible(field.text)
	if field.secret {
		typed = strings.Repeat("•", utf8.RuneCountInString(field.text))
	}
	input := "> " + typed + "_"
	if over := ansi.StringWidth(input) - content; over > 0 {
		input = ansi.TruncateLeft(input, over+1, "…")
	}

	hint := ansi.Truncate(p.hint(keymap.PromptSubmit), content, p.muted().Render("…"))
	lines := []string{title, "", panel.Render(input), "", hint}

	return panel.Padding(1, windowPaddingX).Width(width).Render(strings.Join(lines, "\n"))
}
