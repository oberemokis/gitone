package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// askReset открывает меню режима reset к коммиту (#220), как g в lazygit.
// Hard стирает правки, поэтому спрашивает ещё раз: question.yes выполняет,
// любая другая клавиша нет, как у отмены правок файла.
func (m Model) askReset(target, label string) Model {
	item := func(labelKey string, mode git.ResetMode) menuItem {
		return menuItem{label: m.text.Text(labelKey, nil), run: func(m Model) (tea.Model, tea.Cmd) {
			if mode != git.ResetHard {
				return m.resetTo(target, label, mode)
			}

			args := questionArgs(m.keys)
			args["target"] = label
			return m.ask(m.text.Text("reset.hardQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
				return m.resetTo(target, label, mode)
			}), nil
		}}
	}

	return m.openMenu(m.text.Text("reset.title", i18n.Args{"target": label}), []menuItem{
		item("reset.soft", git.ResetSoft),
		item("reset.mixed", git.ResetMixed),
		item("reset.hard", git.ResetHard),
	})
}

// resetTo переводит текущую ветку на target и закрывает окно. Если reset снял
// коммиты, которые уже лежат в upstream, подвал предупреждает: ветка разошлась
// с remote, и обычный push её не отправит. Пока идёт pull или push, reset
// отказывает.
func (m Model) resetTo(target, label string, mode git.ResetMode) (tea.Model, tea.Cmd) {
	if next, refused := m.refuseWhileRemote(); refused {
		return next, nil
	}

	m.active = noWindow
	dir := m.dir
	note := m.text.Text("reset.done", i18n.Args{"target": label, "mode": string(mode)})
	warning := m.text.Text("reset.published", i18n.Args{"target": label})
	return m, func() tea.Msg {
		published, err := git.Reset(dir, target, mode)
		if !published {
			warning = ""
		}
		return actionDoneMsg{err: err, note: note, warning: warning}
	}
}
