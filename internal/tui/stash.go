package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
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
	case "enter":
		m.active = noWindow
		m.args = []string{ref + "^!"}
		return m.startReload()
	case "a":
		m.active = noWindow
		return m, stashApplyCmd(m.dir, ref)
	case "d":
		m.active = stashDropWindow
	}

	return m, nil
}

// answerStashDrop разбирает ответ на вопрос об удалении. Удаляет только y,
// любая другая клавиша возвращает к списку и сама не выполняется.
func (m Model) answerStashDrop(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.String() != "y" {
		m.active = stashWindow
		return m, nil
	}

	m.active = noWindow
	return m, stashDropCmd(m.dir, m.stashes[m.cursor].Ref)
}

// stashBox рисует список stash. Вопрос об удалении стоит в последней строке
// окна, пока не отвечен.
func stashBox(colors theme.Theme, stashes []git.Stash, cursor int, asking bool, screenWidth, screenHeight int) string {
	rows := make([]string, len(stashes))
	for index, stash := range stashes {
		rows[index] = stash.Ref + "  " + stash.Subject
	}

	box := list{
		title:  "Stash",
		width:  68,
		rows:   rows,
		cursor: cursor,
		empty:  []string{"Нет stash"},
		hint:   "enter диф   a применить   d удалить   esc закрыть",
	}
	if asking {
		box.question = fmt.Sprintf("удалить %s? y да, любая клавиша нет", stashes[cursor].Ref)
	}

	return box.view(colors, screenWidth, screenHeight)
}
