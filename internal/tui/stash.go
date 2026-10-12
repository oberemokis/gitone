package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/tui/theme"
)

// stashesMsg несёт список stash или ошибку git.
type stashesMsg struct {
	stashes []git.Stash
	err     error
}

func stashListCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		stashes, err := git.Stashes(dir)
		return stashesMsg{stashes: stashes, err: err}
	}
}

func stashPushCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashPush(dir)}
	}
}

func stashApplyCmd(dir, ref string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashApply(dir, ref)}
	}
}

func stashDropCmd(dir, ref string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashDrop(dir, ref)}
	}
}

// stashKey отдаёт клавиши окну stash. enter показывает диф stash в экране, a
// применяет его, d спрашивает удаление.
func (m Model) stashKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.stashes)); handled {
		return next, cmd
	}
	if len(m.stashes) == 0 {
		return m, nil
	}

	ref := m.stashes[m.cursor].Ref
	switch key.String() {
	case keyEnter:
		m.active = noWindow
		m.args = []string{ref + "^!"}
		return m.startReload()
	case keyApply:
		m.active = noWindow
		return m, stashApplyCmd(m.dir, ref)
	case keyDrop:
		m.active = stashDropWindow
	}

	return m, nil
}

// answerStashDrop разбирает ответ на вопрос об удалении. Удаляет только y,
// любая другая клавиша возвращает к списку и сама не выполняется.
func (m Model) answerStashDrop(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.String() != keyYes {
		m.active = stashWindow
		return m, nil
	}

	m.active = noWindow
	return m, stashDropCmd(m.dir, m.stashes[m.cursor].Ref)
}

// stashBox рисует список stash. Вопрос об удалении стоит в последней строке
// окна, пока не отвечен.
func stashBox(colors theme.Theme, text i18n.Dictionary, stashes []git.Stash, cursor int, asking bool, screenWidth, screenHeight int) string {
	rows := make([]string, len(stashes))
	for index, stash := range stashes {
		rows[index] = stash.Ref + "  " + stash.Subject
	}

	box := list{
		title:  text.Text("stash.title", nil),
		width:  68,
		rows:   rows,
		cursor: cursor,
		empty:  []string{text.Text("stash.empty", nil)},
		hint:   text.Text("stash.hint", windowKeys()),
	}
	if asking {
		args := windowKeys()
		args["ref"] = stashes[cursor].Ref
		box.question = text.Text("stash.dropQuestion", args)
	}

	return box.view(colors, screenWidth, screenHeight)
}
