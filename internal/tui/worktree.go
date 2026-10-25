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
	repo      git.Repo
	worktrees []git.Worktree
	err       error
}

func (msg worktreesMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.worktreesArrived(msg), nil }
func (msg worktreesMsg) from() git.Repo                     { return msg.repo }

func worktreesCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		worktrees, err := repo.Worktrees()
		return worktreesMsg{repo: repo, worktrees: worktrees, err: err}
	}
}

// worktreesArrived открывает окно worktree с курсором на текущем (arrived).
func (m Model) worktreesArrived(msg worktreesMsg) Model {
	return m.arrived(listArrival{shown: worktreeWindow, err: msg.err, set: func(m *Model) { m.worktrees = msg.worktrees }, start: currentWorktree(msg.worktrees)})
}

// currentWorktree это строка текущего дерева, а без неё первая.
func currentWorktree(worktrees []git.Worktree) int {
	return max(slices.IndexFunc(worktrees, func(w git.Worktree) bool { return w.Current }), 0)
}

// worktreeCheckedMsg несёт открытый репозиторий выбранного дерева или ошибку
// проверки его каталога.
type worktreeCheckedMsg struct {
	repo git.Repo
	err  error
}

func (msg worktreeCheckedMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.worktreeChecked(msg) }

// checkWorktreeCmd проверяет каталог дерева до переключения и открывает его
// репозиторий. Запертое дерево на отключённом диске git держит в списке без
// prunable, и выбор такого дерева оставил бы gitone в каталоге, которого нет.
func checkWorktreeCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if _, err := os.Stat(path); err != nil {
			return worktreeCheckedMsg{err: err}
		}
		repo, err := git.Open(path)
		return worktreeCheckedMsg{repo: repo, err: err}
	}
}

// worktreeChecked переводит gitone на дерево, каталог которого цел. Аргументы
// запуска относятся к прежнему дереву, поэтому после переключения показывается
// дифф рабочего дерева, и b возвращает к нему же. Недоступный каталог оставляет
// прежнее дерево и ставит причину в тост.
//
// Окна, меню, вопросы и окно коммита, открытые, пока шла проверка, показывают
// прежнее дерево, а их действия ушли бы в новое: hard reset на коммит main в
// side-tree, коммит индекса side-tree сообщением для main (#380). Поэтому
// переключение закрывает их. Набранное в окне коммита остаётся в полях.
func (m Model) worktreeChecked(msg worktreeCheckedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.say(m.text.Error(msg.err)), nil
	}

	m.repo = msg.repo.WithAskpass(m.askpass.Env())
	m.args, m.baseArgs = nil, nil
	m.active = noWindow
	m.query, m.searching = "", false
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
