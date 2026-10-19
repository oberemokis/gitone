package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/keymap"
)

// worktreesMsg несёт список рабочих деревьев или ошибку git.
type worktreesMsg struct {
	worktrees []git.Worktree
	err       error
}

func worktreesCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		worktrees, err := git.Worktrees(dir)
		return worktreesMsg{worktrees: worktrees, err: err}
	}
}

// currentWorktree это строка текущего дерева, а без неё первая.
func currentWorktree(worktrees []git.Worktree) int {
	return max(slices.IndexFunc(worktrees, func(w git.Worktree) bool { return w.Current }), 0)
}

// worktreeCheckedMsg несёт итог проверки каталога выбранного дерева: ошибку
// os или nil.
type worktreeCheckedMsg struct {
	path string
	err  error
}

// checkWorktreeCmd проверяет каталог дерева до переключения. Запертое дерево на
// отключённом диске git держит в списке без prunable, и выбор такого дерева
// оставил бы gitone в каталоге, которого нет.
func checkWorktreeCmd(path string) tea.Cmd {
	return func() tea.Msg {
		_, err := os.Stat(path)
		return worktreeCheckedMsg{path: path, err: err}
	}
}

// worktreeChecked переводит gitone на дерево, каталог которого цел. Аргументы
// запуска относятся к прежнему дереву, поэтому после переключения показывается
// дифф рабочего дерева, и b возвращает к нему же. Недоступный каталог оставляет
// прежнее дерево и ставит причину в подвал.
func (m Model) worktreeChecked(msg worktreeCheckedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.say(m.text.Error(msg.err)), nil
	}

	m.dir = msg.path
	m.args, m.baseArgs = nil, nil
	return m.startReload()
}

// worktreeActions это действия окна worktree: worktrees.switch переводит gitone
// на выбранное дерево. Выбор текущего ничего не делает, а каталог выбранного
// проверяется до переключения (worktreeChecked).
var worktreeActions = []windowAction[git.Worktree]{
	{action: keymap.WorktreesSwitch, changesRepo: true, run: func(m Model, worktree git.Worktree) (tea.Model, tea.Cmd) {
		m.active = noWindow
		if worktree.Current {
			return m, nil
		}
		return m, checkWorktreeCmd(worktree.Path)
	}},
}

var worktreeHint = actionsOf(worktreeActions, keymap.ListSearch)

func (m Model) worktreeKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.worktreesShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}
	return dispatch(m, worktreeActions, key.String(), shown)
}

// worktreesShown это деревья под фильтром поиска по ветке и пути, имя
// каталога входит в путь.
func (m Model) worktreesShown() []git.Worktree {
	return matching(m.worktrees, m.query, func(worktree git.Worktree) string {
		return worktree.Branch + "\x00" + worktree.Path
	})
}

// maxNameColumn это предел столбцов имени каталога и ветки в окне worktree:
// путь забирает остаток ширины и не сжимается до нуля из-за длинных имён.
const maxNameColumn = 24

// worktreeBox рисует деревья таблицей, как ветки и лог (#194): имя каталога
// цветом worktree, ветка цветом accent, приглушённый путь, над ними заголовки
// столбцов. Текущее помечено словом сразу за именем каталога, как текущая
// ветка в окне веток. Ширина имени и ветки по самому длинному значению, но не
// больше maxNameColumn, путь занимает остаток и обрезается «…». Каждый кусок
// несёт фон окна сам.
func worktreeBox(p paint, worktrees []git.Worktree, cursor int, query string, typing bool, screenWidth, screenHeight int) string {
	box := list{
		title:  p.text.Text("worktrees.title", nil),
		close:  p.close(keymap.ListClose),
		width:  90,
		cursor: cursor,
		hint:   p.hint(worktreeHint...),
	}.withSearch(p, query, typing, p.text.Text("list.noMatches", nil))

	panel, muted := p.panel(), p.muted()
	current := p.text.Text("worktrees.current", nil)

	titles := []string{p.text.Text("worktrees.column.name", nil), p.text.Text("worktrees.column.branch", nil), p.text.Text("worktrees.column.path", nil)}
	nameWidth, branchWidth := ansi.StringWidth(titles[0]), ansi.StringWidth(titles[1])
	for _, worktree := range worktrees {
		cell := visible(filepath.Base(worktree.Path))
		if worktree.Current {
			cell += " " + current
		}
		nameWidth = max(nameWidth, ansi.StringWidth(cell))
		branchWidth = max(branchWidth, ansi.StringWidth(visible(worktree.Branch)))
	}
	widths := []int{min(nameWidth, maxNameColumn), min(branchWidth, maxNameColumn), 0}
	widths[2] = max(box.rowWidth(screenWidth)-widths[0]-widths[1]-2*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted}, titles...)

	name := panel.Foreground(p.colors.Worktree)
	gap := panel.Render(strings.Repeat(" ", columnGap))
	styles := []lipgloss.Style{panel.Foreground(p.colors.Accent), muted}
	box.rows = make([]string, len(worktrees))
	for index, worktree := range worktrees {
		box.rows[index] = markedCell(panel, name, widths[0], filepath.Base(worktree.Path), worktree.Current, current) + gap + tableRow(panel, widths[1:], styles, worktree.Branch, worktree.Path)
	}

	return box.view(p, screenWidth, screenHeight)
}
