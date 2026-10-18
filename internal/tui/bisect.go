package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// askBisect открывает меню bisect для коммита hash (#245), как b в логе
// lazygit: отметить плохим или хорошим, а во время поиска ещё и сбросить.
// label это короткий хэш или HEAD в строках меню.
func (m Model) askBisect(hash, label string) Model {
	items := []menuItem{
		{label: m.text.Text("bisect.markBad", i18n.Args{"commit": label}), run: func(m Model) (tea.Model, tea.Cmd) {
			return m.markBisect(hash, false)
		}},
		{label: m.text.Text("bisect.markGood", i18n.Args{"commit": label}), run: func(m Model) (tea.Model, tea.Cmd) {
			return m.markBisect(hash, true)
		}},
	}
	if m.location.Operation == "bisect" {
		items = append(items, menuItem{label: m.text.Text("bisect.reset", nil), run: func(m Model) (tea.Model, tea.Cmd) {
			return m.changeHistory(m.text.Text("bisect.resetDone", nil), git.BisectReset)
		}})
	}
	return m.openMenu(m.text.Text("bisect.title", nil), items)
}

// markBisect отмечает коммит и ставит в подвал итог: ждёт второй отметки,
// перешёл на коммит для проверки или нашёл первый плохой. Окно закрывается:
// после отметки HEAD стоит на другом коммите, и дифф перечитывается.
func (m Model) markBisect(hash string, good bool) (tea.Model, tea.Cmd) {
	if next, refused := m.refuseWhileRemote(); refused {
		return next, nil
	}

	m.active = noWindow
	dir, text := m.dir, m.text
	return m, func() tea.Msg {
		step, err := git.BisectMark(dir, hash, good)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{note: bisectNote(text, step)}
	}
}

// bisectNote это итог отметки словами словаря.
func bisectNote(text i18n.Dictionary, step git.BisectStep) string {
	args := i18n.Args{"hash": shortCommit(step.Commit), "subject": visible(step.Subject), "left": step.Left}
	switch {
	case step.Found:
		return text.Text("bisect.found", args)
	case step.Commit != "":
		return text.Text("bisect.next", args)
	}
	return text.Text("bisect.waiting", nil)
}

func shortCommit(hash string) string {
	return hash[:min(len(hash), hashColumn)]
}
