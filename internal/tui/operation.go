package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// askOperation открывает меню незавершённой операции (#243), как m в
// lazygit: продолжить или прервать merge, rebase, cherry-pick или revert.
// Прервать можно только после вопроса: решённые конфликты пропадут вместе с
// операцией. Без операции подвал говорит, что продолжать нечего.
func (m Model) askOperation() Model {
	operation := m.location.Operation
	if operation == "" {
		return m.putNotice(m.text.Text("operation.none", nil), true)
	}

	state := m.text.Text("operation."+operation, nil)
	return m.openMenu(m.text.Text("operation.title", i18n.Args{"state": state}), []menuItem{
		{label: m.text.Text("operation.continue", nil), run: func(m Model) (tea.Model, tea.Cmd) {
			return m.changeHistory(m.text.Text("operation.continued", nil), git.Continue)
		}},
		{label: m.text.Text("operation.abort", nil), run: func(m Model) (tea.Model, tea.Cmd) {
			args := questionArgs(m.keys)
			args["state"] = state
			return m.ask(m.text.Text("operation.abortQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
				return m.changeHistory(m.text.Text("operation.aborted", nil), git.Abort)
			}), nil
		}},
	})
}
