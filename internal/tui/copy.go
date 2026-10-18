package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// patchMsg несёт сохранённый патч коммита: имя файла от корня и текст, или
// ошибку git.
type patchMsg struct {
	name  string
	patch string
	err   error
}

// askCopy открывает меню копирования коммита (#224), как y в lazygit: хэш в
// буфер обмена, патч в буфер и в файл. Буфер это OSC 52: терминал не отвечает,
// принял ли он текст, поэтому патч сохраняется файлом всегда.
func (m Model) askCopy(entry git.LogEntry) Model {
	short := entry.Hash[:7]
	return m.openMenu(m.text.Text("copy.title", i18n.Args{"hash": short}), []menuItem{
		{label: m.text.Text("copy.hash", nil), run: func(m Model) (tea.Model, tea.Cmd) {
			m = m.sayCalm(m.text.Text("copy.hashDone", i18n.Args{"hash": short}))
			return m, tea.SetClipboard(entry.Hash)
		}},
		{label: m.text.Text("copy.patch", nil), run: func(m Model) (tea.Model, tea.Cmd) {
			dir := m.dir
			return m, func() tea.Msg {
				name, patch, err := git.Patch(dir, entry.Hash)
				return patchMsg{name: name, patch: patch, err: err}
			}
		}},
	})
}

// patchSaved кладёт патч в буфер обмена и называет файл в подвале. Файл
// появился в рабочем дереве, поэтому дифф перечитывается.
func (m Model) patchSaved(msg patchMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.say(m.text.Error(msg.err)), nil
	}

	m = m.sayCalm(m.text.Text("copy.patchDone", i18n.Args{"name": visible(msg.name)}))
	next, reload := m.startReload()
	return next, tea.Batch(tea.SetClipboard(msg.patch), reload)
}
