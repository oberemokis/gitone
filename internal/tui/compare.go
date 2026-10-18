package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// comparison это точка сравнения (#216): ревизия для git и её подпись для
// заголовка, имя ветки или короткий хэш.
type comparison struct {
	ref   string
	label string
}

// compareWith показывает дифф от ref до рабочего дерева, как W в lazygit:
// `git diff <ref> --`. Это вид только для просмотра, как дифф коммита из лога:
// space и x в нём отказывают, diff.workingTree возвращает к диффу запуска.
func (m Model) compareWith(ref, label string) (tea.Model, tea.Cmd) {
	m.active = noWindow
	m.args = []string{ref, "--"}
	m.compare = comparison{ref: ref, label: label}
	m = m.putNotice(m.text.Text("compare.started", i18n.Args{"ref": visible(label), "key": firstKey(m.keys, keymap.DiffWorkingTree)}), true)
	return m.startReload()
}

// compareLabel это подпись сравнения для заголовка, пока на экране дифф от
// точки сравнения, иначе пустая строка: лог, stash и b меняют аргументы, и
// подпись уходит вместе с ними.
func (m Model) compareLabel() string {
	if m.compare.ref == "" || !slices.Equal(m.args, []string{m.compare.ref, "--"}) {
		return ""
	}
	return m.text.Text("compare.label", i18n.Args{"ref": visible(m.compare.label)})
}

// viewLabel это всё, что заголовок говорит о виде диффа: сравнение и флаги.
func (m Model) viewLabel() string {
	var parts []string
	for _, part := range []string{m.compareLabel(), m.flagsLabel()} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ", ")
}
