package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
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

// branchKey отдаёт клавиши окну веток: branches.switch переключает на
// выбранную, list.search открывает поиск, list.close закрывает. Переключение
// на текущую ничего не делает.
func (m Model) branchKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.branchesShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}
	if !m.keys.Is(keymap.BranchesSwitch, key.String()) || len(shown) == 0 {
		return m, nil
	}

	branch := shown[m.cursor]
	m.active = noWindow
	if branch.Current {
		return m, nil
	}
	return m, switchCmd(m.dir, branch.Name)
}

// branchesShown это ветки под фильтром поиска по имени.
func (m Model) branchesShown() []git.Branch {
	return matching(m.branches, m.query, func(branch git.Branch) string { return branch.Name })
}

// branchBox рисует ветки таблицей, как лог (#124): имя, автор и время
// последнего коммита, над ними заголовки столбцов. Имя цветом accent, как
// ветка в окне worktree (#117), автор и время приглушённые. Столбец имени
// забирает остаток ширины, автор и время стоят у правого края, как в логе
// (#131). Текущая помечена словом сразу за именем, а не цветом: так видно и
// без цветов.
func branchBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, branches []git.Branch, cursor int, query string, typing bool, screenWidth, screenHeight int) string {
	box := list{
		title:  text.Text("branches.title", nil),
		close:  closeHint(colors, keys, text, keymap.ListClose),
		width:  90,
		cursor: cursor,
		hint:   windowHint(colors, keys, text, keymap.BranchesSwitch, keymap.ListSearch),
	}.withSearch(colors, text, keys, query, typing, text.Text("list.noMatches", nil))

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)
	current := text.Text("branches.current", nil)

	titles := []string{text.Text("branches.column.name", nil), text.Text("branches.column.author", nil), text.Text("branches.column.date", nil)}
	authorWidth, dateWidth := ansi.StringWidth(titles[1]), ansi.StringWidth(titles[2])
	for _, branch := range branches {
		authorWidth = max(authorWidth, ansi.StringWidth(cellText(branch.Author)))
		dateWidth = max(dateWidth, ansi.StringWidth(branch.When))
	}
	widths := []int{0, min(authorWidth, maxAuthorColumn), min(dateWidth, maxDateColumn)}
	widths[0] = max(box.rowWidth(screenWidth)-widths[1]-widths[2]-2*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted}, titles...)

	name := panel.Foreground(colors.Accent)
	gap := panel.Render(strings.Repeat(" ", columnGap))
	box.rows = make([]string, len(branches))
	for index, branch := range branches {
		box.rows[index] = branchCell(panel, name, widths[0], branch, current) + gap + tableRow(panel, widths[1:], []lipgloss.Style{muted, muted}, branch.Author, branch.When)
	}

	return box.view(colors, screenWidth, screenHeight)
}

// branchCell это ячейка имени шириной width: имя цветом name, у текущей за
// ним через пробел отметка цветом текста. Длинное имя обрезается «…», отметка
// остаётся видна.
func branchCell(panel, name lipgloss.Style, width int, branch git.Branch, current string) string {
	mark := ""
	if branch.Current {
		mark = " " + current
	}

	label := ansi.Truncate(branch.Name, max(width-ansi.StringWidth(mark), 1), "…")
	used := ansi.StringWidth(label) + ansi.StringWidth(mark)
	return name.Render(label) + panel.Render(mark+strings.Repeat(" ", max(width-used, 0)))
}
