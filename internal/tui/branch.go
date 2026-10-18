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

// createBranchCmd создаёт ветку name на base и переключается на неё: дифф и
// подвал перечитываются, как после switch.
func createBranchCmd(dir, name, base string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.CreateBranch(dir, name, base)}
	}
}

// deleteBranchCmd удаляет ветку и отвечает новым списком: окно веток остаётся
// открытым.
func deleteBranchCmd(dir, name string) tea.Cmd {
	return thenBranches(dir, func() error { return git.DeleteBranch(dir, name) })
}

func renameBranchCmd(dir, from, to string) tea.Cmd {
	return thenBranches(dir, func() error { return git.RenameBranch(dir, from, to) })
}

// thenBranches выполняет правку веток и отвечает новым списком, а отказ git
// ошибкой: окно веток остаётся открытым, корень перечитывает подвал.
func thenBranches(dir string, change func() error) tea.Cmd {
	return func() tea.Msg {
		if err := change(); err != nil {
			return branchesMsg{err: err}
		}
		return branchesCmd(dir)()
	}
}

// branchKey отдаёт клавиши окну веток: branches.switch переключает на
// выбранную, branches.new создаёт ветку на выбранной, branches.delete удаляет
// выбранную после вопроса, list.search открывает поиск, list.close закрывает.
// Переключение на текущую ничего не делает.
func (m Model) branchKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.branchesShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}

	pressed := key.String()
	if m.keys.Is(keymap.CherryPickPaste, pressed) {
		return m.pasteCommit()
	}
	acts := []keymap.Action{keymap.BranchesSwitch, keymap.BranchesNew, keymap.BranchesRename, keymap.BranchesDelete, keymap.BranchesMerge, keymap.BranchesRebase}
	if !slices.ContainsFunc(acts, func(action keymap.Action) bool { return m.keys.Is(action, pressed) }) || len(shown) == 0 {
		return m, nil
	}
	if next, refused := m.refuseWhileRemote(); refused {
		return next, nil
	}

	branch := shown[m.cursor]
	switch {
	case m.keys.Is(keymap.BranchesNew, pressed):
		return m.askBranchName(branch.Name), nil
	case m.keys.Is(keymap.BranchesRename, pressed):
		return m.askRename(branch.Name), nil
	case m.keys.Is(keymap.BranchesDelete, pressed):
		return m.askDeleteBranch(branch), nil
	case m.keys.Is(keymap.BranchesMerge, pressed):
		return m.ontoCurrent(branch, git.Merge, "branches.merged")
	case m.keys.Is(keymap.BranchesRebase, pressed):
		return m.ontoCurrent(branch, git.Rebase, "branches.rebased")
	}

	m.active = noWindow
	if branch.Current {
		return m, nil
	}
	return m, switchCmd(m.dir, branch.Name)
}

// ontoCurrent сливает выбранную ветку в текущую или перестраивает текущую
// поверх неё (#229), без вопроса: операцию отменяет reflog, а конфликт
// останавливает её с причиной в подвале. Выбрана сама текущая, и подвал
// сразу говорит, что нужна другая. Итог успеха noteKey гаснет сам.
func (m Model) ontoCurrent(branch git.Branch, run func(dir, name string) error, noteKey string) (tea.Model, tea.Cmd) {
	if branch.Current {
		return m.putNotice(m.text.Text("branches.sameBranch", nil), false), nil
	}

	m.active = noWindow
	dir, note := m.dir, m.text.Text(noteKey, i18n.Args{"name": visible(branch.Name)})
	return m, func() tea.Msg {
		return actionDoneMsg{err: run(dir, branch.Name), note: note}
	}
}

// askBranchName спрашивает имя новой ветки на base, как n в lazygit: базой
// служит выбранная ветка, а курсор при открытии окна стоит на текущей.
func (m Model) askBranchName(base string) Model {
	title := m.text.Text("branches.newTitle", i18n.Args{"base": visible(base)})
	return m.openPrompt(title, "", func(m Model, name string) (tea.Model, tea.Cmd) {
		m.active = noWindow
		return m, createBranchCmd(m.dir, name, base)
	})
}

// askRename спрашивает новое имя ветки, в поле стоит старое, как R в lazygit.
// То же имя ничего не делает. Окно веток остаётся открытым со свежим списком.
func (m Model) askRename(from string) Model {
	title := m.text.Text("branches.renameTitle", i18n.Args{"name": visible(from)})
	return m.openPrompt(title, from, func(m Model, to string) (tea.Model, tea.Cmd) {
		if to == from {
			return m, nil
		}
		return m, renameBranchCmd(m.dir, from, to)
	})
}

// askDeleteBranch спрашивает, удалять ли ветку. Текущую git не удалит, поэтому
// вопрос не задаётся: подвал сразу говорит почему.
func (m Model) askDeleteBranch(branch git.Branch) Model {
	if branch.Current {
		return m.putNotice(m.text.Text("branches.deleteCurrent", nil), false)
	}

	args := questionArgs(m.keys)
	args["name"] = visible(branch.Name)
	return m.ask(m.text.Text("branches.deleteQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
		return m, deleteBranchCmd(m.dir, branch.Name)
	})
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
func branchBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, branches []git.Branch, cursor int, query string, typing bool, question string, screenWidth, screenHeight int) string {
	box := list{
		title:    text.Text("branches.title", nil),
		close:    closeHint(colors, keys, text, keymap.ListClose),
		width:    90,
		cursor:   cursor,
		empty:    []string{text.Text("branches.empty", nil)},
		hint:     windowHint(colors, keys, text, keymap.BranchesSwitch, keymap.BranchesNew, keymap.BranchesRename, keymap.BranchesDelete, keymap.BranchesMerge, keymap.BranchesRebase, keymap.CherryPickPaste, keymap.ListSearch),
		question: question,
	}.withSearch(colors, text, keys, query, typing, text.Text("list.noMatches", nil))

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)
	current := text.Text("branches.current", nil)

	titles := []string{text.Text("branches.column.name", nil), text.Text("branches.column.author", nil), text.Text("branches.column.date", nil)}
	authorWidth, dateWidth := ansi.StringWidth(titles[1]), ansi.StringWidth(titles[2])
	for _, branch := range branches {
		authorWidth = max(authorWidth, ansi.StringWidth(visible(branch.Author)))
		dateWidth = max(dateWidth, ansi.StringWidth(visible(branch.When)))
	}
	widths := []int{0, min(authorWidth, maxAuthorColumn), min(dateWidth, maxDateColumn)}
	widths[0] = max(box.rowWidth(screenWidth)-widths[1]-widths[2]-2*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted}, titles...)

	name := panel.Foreground(colors.Accent)
	gap := panel.Render(strings.Repeat(" ", columnGap))
	box.rows = make([]string, len(branches))
	for index, branch := range branches {
		box.rows[index] = markedCell(panel, name, widths[0], branch.Name, branch.Current, current) + gap + tableRow(panel, widths[1:], []lipgloss.Style{muted, muted}, branch.Author, branch.When)
	}

	return box.view(colors, screenWidth, screenHeight)
}

// markedCell это ячейка имени шириной width для окон веток и worktree: имя
// цветом name, у текущей за ним через пробел отметка mark цветом текста. Длинное
// имя обрезается «…», отметка остаётся видна.
func markedCell(panel, name lipgloss.Style, width int, label string, current bool, mark string) string {
	if current {
		mark = " " + mark
	} else {
		mark = ""
	}

	label = ansi.Truncate(visible(label), max(width-ansi.StringWidth(mark), 1), "…")
	used := ansi.StringWidth(label) + ansi.StringWidth(mark)
	return name.Render(label) + panel.Render(mark+strings.Repeat(" ", max(width-used, 0)))
}
