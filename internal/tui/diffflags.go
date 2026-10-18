package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/i18n"
)

// gitContext это число строк контекста у git по умолчанию. От него идут
// diff.moreContext и diff.lessContext, пока число не меняли.
// ponytail: diff.context из конфига пользователя не читается, первое нажатие
// считает от трёх; читать его стоит, если окажется, что его задают.
const gitContext = 3

// diffFlags это флаги вида диффа (#217): без пробелов и число строк контекста.
// Без нажатий флагов нет, и git берёт свои настройки.
func (m Model) diffFlags() []string {
	var flags []string
	if m.ignoreSpace {
		flags = append(flags, "--ignore-all-space")
	}
	if m.context >= 0 {
		flags = append(flags, "--unified="+strconv.Itoa(m.context))
	}
	return flags
}

// toggleWhitespace прячет и возвращает правки из одних пробелов. Значение
// держится до выхода из gitone, в конфиг не пишется.
func (m Model) toggleWhitespace() (tea.Model, tea.Cmd) {
	m.ignoreSpace = !m.ignoreSpace
	return m.flagsChanged()
}

// changeContext меняет число строк контекста вокруг хунков на delta, не меньше
// нуля.
func (m Model) changeContext(delta int) (tea.Model, tea.Cmd) {
	lines := m.context
	if lines < 0 {
		lines = gitContext
	}
	lines = max(lines+delta, 0)
	if lines == m.context {
		return m, nil
	}

	m.context = lines
	return m.flagsChanged()
}

// flagsChanged перечитывает дифф с новыми флагами, подвал называет вид. Без
// флагов вид остаётся только один: пробелы снова видны.
func (m Model) flagsChanged() (tea.Model, tea.Cmd) {
	view := m.flagsLabel()
	if view == "" {
		view = m.text.Text("diff.whitespaceShown", nil)
	}

	m = m.sayCalm(m.text.Text("diff.flagsChanged", i18n.Args{"view": view}))
	return m.startReload()
}

// flagsLabel это вид диффа словами для заголовка и подвала: «whitespace
// hidden, context 5». Без флагов пустая строка.
func (m Model) flagsLabel() string {
	var parts []string
	if m.ignoreSpace {
		parts = append(parts, m.text.Text("diff.noWhitespace", nil))
	}
	if m.context >= 0 {
		parts = append(parts, m.text.Text("diff.context", i18n.Args{"lines": m.context}))
	}
	return strings.Join(parts, ", ")
}
