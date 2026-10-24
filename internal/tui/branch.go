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
)

// branchesMsg несёт список локальных веток или ошибку git. edit значит, что
// список пришёл после правки, note это её итог.
type branchesMsg struct {
	branches []git.Branch
	err      error
	edit     bool
	note     string
}

func (msg branchesMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.branchesArrived(msg) }

func branchesCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		branches, err := repo.Branches()
		return branchesMsg{branches: branches, err: err}
	}
}

// branchesArrived открывает окно веток с курсором на текущей или обновляет
// открытое (arrived). После удаления или переименования подвал говорит итог
// и перечитывается: имя текущей ветки могло смениться.
func (m Model) branchesArrived(msg branchesMsg) (tea.Model, tea.Cmd) {
	m = m.arrived(listArrival{shown: branchWindow, err: msg.err, edit: msg.edit, set: func(m *Model) { m.branches = msg.branches }, start: currentIndex(msg.branches)})
	if msg.err != nil || !msg.edit {
		return m, nil
	}

	if msg.note != "" {
		m = m.sayCalm(msg.note)
	}
	return m.startReload()
}

// switchCmd переключает ветку. Отказ git приходит ошибкой и остаётся в подвале.
func switchCmd(repo git.Repo, name string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: repo.Switch(name)}
	}
}

// currentIndex это строка текущей ветки, а без неё (detached HEAD) первая.
func currentIndex(branches []git.Branch) int {
	return max(slices.IndexFunc(branches, func(b git.Branch) bool { return b.Current }), 0)
}

// createBranchCmd создаёт ветку name на base и переключается на неё: дифф и
// подвал перечитываются, как после switch.
func createBranchCmd(repo git.Repo, name, base string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: repo.CreateBranch(name, base)}
	}
}

// deleteBranchCmd удаляет ветку и отвечает новым списком: окно веток остаётся
// открытым.
func deleteBranchCmd(repo git.Repo, name string) tea.Cmd {
	return thenBranches(repo, func() error { return repo.DeleteBranch(name) })
}

func renameBranchCmd(repo git.Repo, from, to string) tea.Cmd {
	return thenBranches(repo, func() error { return repo.RenameBranch(from, to) })
}

// thenBranches выполняет правку веток и отвечает новым списком, а отказ git
// ошибкой: окно веток остаётся открытым, корень перечитывает подвал.
func thenBranches(repo git.Repo, change func() error) tea.Cmd {
	return func() tea.Msg {
		if err := change(); err != nil {
			return branchesMsg{err: err}
		}
		msg := branchesCmd(repo)().(branchesMsg)
		msg.edit = true
		return msg
	}
}

// branchActions это действия окна веток: branches.switch переключает на
// выбранную (на текущую ничего не делает), branches.new создаёт ветку на
// выбранной, branches.delete удаляет выбранную после вопроса.
var branchActions = []windowAction[git.Branch]{
	{action: keymap.BranchesSwitch, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		m.active = noWindow
		if branch.Current {
			return m, nil
		}
		return m, switchCmd(m.repo, branch.Name)
	}},
	{action: keymap.BranchesNew, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m.askBranchName(branch.Name, branch.Name), nil
	}},
	{action: keymap.BranchesRename, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m.askRename(branch.Name), nil
	}},
	{action: keymap.BranchesDelete, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m.askDeleteBranch(branch), nil
	}},
	{action: keymap.BranchesMerge, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m.ontoCurrent(branch, git.Repo.Merge, "branches.merged")
	}},
	{action: keymap.BranchesRebase, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m.ontoCurrent(branch, git.Repo.Rebase, "branches.rebased")
	}},
	{action: keymap.BranchesReset, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m, resetTargetCmd(m.repo, branch)
	}},
	{action: keymap.BranchesUpstream, changesRepo: true, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m, remoteBranchesCmd(m.repo, branch.Name)
	}},
	{action: keymap.CherryPickPaste, anyRow: true, run: func(m Model, _ git.Branch) (tea.Model, tea.Cmd) {
		return m.pasteCommit()
	}},
	{action: keymap.CompareWith, run: func(m Model, branch git.Branch) (tea.Model, tea.Cmd) {
		return m.compareWith(branch.Name, branch.Name)
	}},
}

var branchHint = actionsOf(branchActions, keymap.ListSearch)

func (m Model) branchKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.branchesShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}
	return dispatch(m, branchActions, key.String(), shown)
}

// ontoCurrent сливает выбранную ветку в текущую или перестраивает текущую
// поверх неё (#229), без вопроса: операцию отменяет reflog, а конфликт
// останавливает её с причиной в подвале. Выбрана сама текущая, и подвал
// сразу говорит, что нужна другая. Итог успеха noteKey гаснет сам.
func (m Model) ontoCurrent(branch git.Branch, run func(repo git.Repo, name string) error, noteKey string) (tea.Model, tea.Cmd) {
	if branch.Current {
		return m.putNotice(m.text.Text("branches.sameBranch", nil)), nil
	}

	m.active = noWindow
	repo, note := m.repo, m.text.Text(noteKey, i18n.Args{"name": visible(branch.Name)})
	return m, func() tea.Msg {
		return actionDoneMsg{err: run(repo, branch.Name), note: note}
	}
}

// askBranchName спрашивает имя новой ветки на base, как n в lazygit: в окне
// веток базой служит выбранная ветка, а курсор при открытии окна стоит на
// текущей; в логе выбранный коммит (#223). label это база в заголовке поля:
// имя ветки или короткий хэш.
func (m Model) askBranchName(label, base string) Model {
	title := m.text.Text("branches.newTitle", i18n.Args{"base": visible(label)})
	return m.openPrompt(title, "", func(m Model, name string) (tea.Model, tea.Cmd) {
		m.active = noWindow
		return m, createBranchCmd(m.repo, name, base)
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
		return m, renameBranchCmd(m.repo, from, to)
	})
}

// askDeleteBranch спрашивает, удалять ли ветку. Текущую git не удалит, поэтому
// вопрос не задаётся: подвал сразу говорит почему.
func (m Model) askDeleteBranch(branch git.Branch) Model {
	if branch.Current {
		return m.putNotice(m.text.Text("branches.deleteCurrent", nil))
	}

	args := questionArgs(m.keys)
	args["name"] = visible(branch.Name)
	return m.ask(m.text.Text("branches.deleteQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
		return m, deleteBranchCmd(m.repo, branch.Name)
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
func branchBox(p paint, branches []git.Branch, cursor int, query string, typing bool, question string, screenWidth, screenHeight int) string {
	box := list{
		title:    p.text.Text("branches.title", nil),
		close:    p.close(keymap.ListClose),
		width:    90,
		cursor:   cursor,
		empty:    []string{p.text.Text("branches.empty", nil)},
		hint:     p.hint(branchHint...),
		question: question,
	}.withSearch(p, query, typing, p.text.Text("list.noMatches", nil))

	panel, muted := p.panel(), p.muted()
	current := p.text.Text("branches.current", nil)

	titles := []string{p.text.Text("branches.column.name", nil), p.text.Text("branches.column.author", nil), p.text.Text("branches.column.date", nil)}
	authorWidth, dateWidth := ansi.StringWidth(titles[1]), ansi.StringWidth(titles[2])
	for _, branch := range branches {
		authorWidth = max(authorWidth, ansi.StringWidth(visible(branch.Author)))
		dateWidth = max(dateWidth, ansi.StringWidth(visible(branch.When)))
	}
	widths := []int{0, min(authorWidth, maxAuthorColumn), min(dateWidth, maxDateColumn)}
	widths[0] = max(box.rowWidth(screenWidth)-widths[1]-widths[2]-2*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted}, titles...)

	name := panel.Foreground(p.colors.Accent)
	gap := panel.Render(strings.Repeat(" ", columnGap))
	box.rows = make([]string, len(branches))
	for index, branch := range branches {
		box.rows[index] = markedCell(panel, name, widths[0], branch.Name, branch.Current, current) + gap + tableRow(panel, widths[1:], []lipgloss.Style{muted, muted}, branch.Author, branch.When)
	}

	return box.view(p, screenWidth, screenHeight)
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
