package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// resetTargetMsg несёт цель reset из окна веток и число коммитов, которые он
// снимет, или ошибку: у текущей ветки нет upstream.
type resetTargetMsg struct {
	target string
	drops  int
	err    error
}

// resetTargetCmd находит цель reset из окна веток (#231): у текущей ветки её
// upstream, у другой она сама, и считает снимаемые коммиты.
func resetTargetCmd(dir string, branch git.Branch) tea.Cmd {
	return func() tea.Msg {
		target, drops, err := git.ResetTarget(dir, branch.Name, branch.Current)
		return resetTargetMsg{target: target, drops: drops, err: err}
	}
}

// resetTargetFound открывает меню режимов поверх окна веток, если оно ещё
// открыто: ответ git мог прийти после того, как окно закрыли.
func (m Model) resetTargetFound(msg resetTargetMsg) Model {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case m.active != branchWindow:
		return m
	}
	return m.askReset(msg.target, msg.target, msg.drops)
}

// askReset открывает меню режима reset к коммиту (#220) или ветке (#231), как g
// в lazygit. Hard стирает правки, поэтому спрашивает ещё раз: question.yes
// выполняет, любая другая клавиша нет, как у отмены правок файла. drops это
// число коммитов, которые снимет reset, отрицательное значит, что его не
// считали: тогда вопрос его не называет.
func (m Model) askReset(target, label string, drops int) Model {
	item := func(labelKey string, mode git.ResetMode) menuItem {
		return menuItem{label: m.text.Text(labelKey, nil), run: func(m Model) (tea.Model, tea.Cmd) {
			if mode != git.ResetHard {
				return m.resetTo(target, label, mode)
			}

			args := questionArgs(m.keys)
			args["target"], args["count"] = label, drops
			question := "reset.hardQuestion"
			if drops >= 0 {
				question = "reset.hardQuestionDrops"
			}
			return m.ask(m.text.Text(question, args), func(m Model) (tea.Model, tea.Cmd) {
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
