package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/askpass"
)

// Вопросы git о логине и пароле (#375). Сетевая команда git спрашивает
// программу из GIT_ASKPASS, ею служит сам gitone (askpass), а вопрос приходит
// сюда и открывается в поле ввода. Ответ или отмена уходят git обратно.

// credentialMsg несёт вопрос git.
type credentialMsg struct{ request askpass.Request }

// apply запоминает вопрос, а открывает его followCredential: поле ввода может
// быть занято.
func (msg credentialMsg) apply(m Model) (tea.Model, tea.Cmd) {
	m.credential = &msg.request
	m.prompt.credential = false
	return m, nil
}

// waitCredential ждёт следующий вопрос. Ожидание возобновляется, когда на
// вопрос ответили или его отменили: вопросы идут по одному. Без сервера ждать
// нечего.
func waitCredential(server *askpass.Server) tea.Cmd {
	if server == nil {
		return nil
	}

	return func() tea.Msg {
		return credentialMsg{<-server.Requests()}
	}
}

// followCredential держит вопрос git на экране, пока на него не ответили.
// Вопрос, пришедший при открытом поле ввода, ждёт его закрытия и не затирает
// набранное. Поле вопроса, закрытое не ответом (esc, клик мимо, другое окно),
// отменяет вопрос вместе с сетевой операцией: иначе git ждал бы ответа,
// который никто не даст. Операция идёт одна (remote.go), поэтому вопрос задала
// она, и тост говорит «Cancelled», а не «unable to read askpass response».
func (m Model) followCredential(cmd tea.Cmd) (Model, tea.Cmd) {
	switch {
	case m.credential == nil, m.active == promptWindow && m.prompt.credential:
		return m, cmd
	case m.prompt.credential:
		m.credential.Cancel()
		m.credential, m.prompt.credential = nil, false
		return m.stopRemote(), tea.Batch(cmd, waitCredential(m.askpass))
	case m.active == promptWindow:
		return m, cmd
	}

	return m.openCredential(), cmd
}

// openCredential открывает вопрос в поле ввода. Заголовок это вопрос git без
// двоеточия: «Username for 'https://host'». Всё, кроме имени, набирается
// точками: пароль, токен, passphrase и вопросы, которых gitone не знает.
func (m Model) openCredential() Model {
	request := *m.credential
	title := strings.TrimSuffix(strings.TrimSpace(request.Prompt), ":")
	m = m.openPrompt(title, "", func(m Model, text string) (tea.Model, tea.Cmd) {
		request.Answer(text)
		m.credential, m.prompt.credential = nil, false
		return m, waitCredential(m.askpass)
	})
	m.prompt.empty = true
	m.prompt.secret = !strings.HasPrefix(request.Prompt, "Username")
	m.prompt.credential = true
	return m
}
