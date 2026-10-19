package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// undoStepMsg несёт найденный шаг отката или повтора либо отказ: откатывать
// нечего, последним был stash, идёт rebase.
type undoStepMsg struct {
	step git.Step
	err  error
}

func (msg undoStepMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.askUndo(msg), nil }

// undoStepCmd ищет шаг по reflog в команде: чтение reflog это IO, ему не место
// в Update.
func undoStepCmd(repo git.Repo, redo bool) tea.Cmd {
	find := repo.UndoStep
	if redo {
		find = repo.RedoStep
	}
	return func() tea.Msg {
		step, err := find()
		return undoStepMsg{step: step, err: err}
	}
}

// askUndo спрашивает в подвале, откатить или повторить найденный шаг (#239):
// вопрос называет запись reflog и то, куда встанет HEAD. Ошибка ставится
// причиной в подвал. Окно, открытое, пока шаг искался, вопрос не перебивает.
func (m Model) askUndo(msg undoStepMsg) Model {
	if msg.err != nil {
		return m.say(m.text.Error(msg.err))
	}
	if m.active != noWindow {
		return m
	}

	step := msg.step
	target := step.Target
	if len(target) == 40 {
		target = target[:7]
	}
	args := questionArgs(m.keys)
	args["action"], args["subject"], args["target"] = visible(step.Action), visible(step.Subject), visible(target)

	// шаг checkout переключает ветку, и файлы становятся как в ней: вопрос
	// «файлы остаются» здесь неправда
	question, note := "undo.question", "undo.done"
	switch {
	case step.Redo && step.Checkout:
		question, note = "redo.questionCheckout", "redo.done"
	case step.Redo:
		question, note = "redo.question", "redo.done"
	case step.Checkout:
		question = "undo.questionCheckout"
	}
	done := m.text.Text(note, i18n.Args{"target": visible(target)})
	return m.ask(m.text.Text(question, args), func(m Model) (tea.Model, tea.Cmd) {
		return m.changeHistory(done, func(repo git.Repo) error { return repo.ApplyStep(step) })
	})
}
