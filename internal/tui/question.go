package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/keymap"
)

// question это вопрос перед необратимым действием (#227): удаление ветки,
// hard reset. yes выполняется по question.yes, любая другая клавиша, ctrl+c
// тоже, возвращает в окно back и ничего не делает. Вопрос из окна стоит
// последней строкой окна, как удаление stash, вопрос с главного экрана в
// подвале, как discard.
type question struct {
	text string
	yes  func(m Model) (tea.Model, tea.Cmd)
	back window
}

// ask задаёт вопрос text поверх открытого окна или главного экрана.
func (m Model) ask(text string, yes func(m Model) (tea.Model, tea.Cmd)) Model {
	m.question = question{text: text, yes: yes, back: m.active}
	m.active = questionWindow
	return m
}

// answerQuestion разбирает ответ: окно под вопросом возвращается в любом
// случае, действие выполняется только по question.yes.
func (m Model) answerQuestion(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	asked := m.question
	m.question = question{}
	m.active = asked.back

	if !m.keys.Is(keymap.QuestionYes, key.String()) {
		return m, nil
	}
	return asked.yes(m)
}

// asking это окно, которое видно на экране, и текст вопроса над ним, если вопрос
// задан.
func (m Model) asking() (shown window, text string) {
	if m.active == questionWindow {
		return m.question.back, m.question.text
	}
	return m.active, ""
}

// beneath это окно под вопросом, меню или полем ввода, а без них открытое:
// ответ правки, пришедший, пока над списком висит вопрос, обновляет список под
// ним. У меню и поля курсор свой, курсор списка они не трогают.
func (m Model) beneath() window {
	switch m.active {
	case questionWindow:
		return m.question.back
	case menuWindow:
		return m.menu.back
	case promptWindow:
		return m.prompt.back
	case stashDropWindow:
		return stashWindow
	}
	return m.active
}

// footerQuestion говорит, что вопрос задан с главного экрана и стоит в
// подвале: под ним видны дифф и справка, но клики из них не принимаются, ответ
// идёт клавишей.
func (m Model) footerQuestion() bool {
	return m.active == questionWindow && m.question.back == noWindow
}
