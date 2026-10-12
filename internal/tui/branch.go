package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/tui/theme"
)

// branchesMsg несёт список локальных веток или ошибку git.
type branchesMsg struct {
	branches []git.Branch
	err      error
}

func branchesCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		branches, err := git.Branches(dir)
		return branchesMsg{branches: branches, err: err}
	}
}

// switchCmd переключает ветку. Отказ git приходит ошибкой и остаётся в подвале.
func switchCmd(dir, name string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.Switch(dir, name)}
	}
}

// currentIndex это строка текущей ветки, а без неё (detached HEAD) первая.
func currentIndex(branches []git.Branch) int {
	return max(slices.IndexFunc(branches, func(b git.Branch) bool { return b.Current }), 0)
}

// branchKey отдаёт клавиши окну веток: enter переключает на выбранную, esc
// закрывает. Переключение на текущую ничего не делает.
func (m Model) branchKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.branches)); handled {
		return next, cmd
	}
	if key.String() != keyEnter || len(m.branches) == 0 {
		return m, nil
	}

	branch := m.branches[m.cursor]
	m.active = noWindow
	if branch.Current {
		return m, nil
	}
	return m, switchCmd(m.dir, branch.Name)
}

// branchBox рисует список веток. Текущая помечена словом, а не цветом: так
// видно и без цветов.
func branchBox(colors theme.Theme, text i18n.Dictionary, branches []git.Branch, cursor, screenWidth, screenHeight int) string {
	rows := make([]string, len(branches))
	for index, branch := range branches {
		rows[index] = branch.Name
		if branch.Current {
			rows[index] += " " + text.Text("branches.current", nil)
		}
	}

	return list{
		title:  text.Text("branches.title", nil),
		width:  60,
		rows:   rows,
		cursor: cursor,
		hint:   text.Text("branches.hint", windowKeys()),
	}.view(colors, screenWidth, screenHeight)
}
