package tui

import (
	"os"
	"path/filepath"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
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

// worktreeKey отдаёт клавиши окну worktree: worktrees.switch переводит gitone
// на выбранное дерево, list.search открывает поиск, list.close закрывает.
// Выбор текущего ничего не делает, а каталог выбранного проверяется до
// переключения (worktreeChecked).
func (m Model) worktreeKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.worktreesShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}
	if !m.keys.Is(keymap.WorktreesSwitch, key.String()) || len(shown) == 0 {
		return m, nil
	}

	worktree := shown[m.cursor]
	m.active = noWindow
	if worktree.Current {
		return m, nil
	}

	return m, checkWorktreeCmd(worktree.Path)
}

// worktreesShown это деревья под фильтром поиска по ветке и пути, имя
// каталога входит в путь.
func (m Model) worktreesShown() []git.Worktree {
	return matching(m.worktrees, m.query, func(worktree git.Worktree) string {
		return worktree.Branch + "\x00" + worktree.Path
	})
}

// worktreeBox рисует список деревьев: имя каталога, ветка цветом accent и
// приглушённый путь (#109). Текущее помечено словом, как ветка в окне веток.
// Отметка стоит до пути: длинный путь обрезается по краю окна и унёс бы её с
// собой. Каждый кусок несёт фон окна сам.
func worktreeBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, worktrees []git.Worktree, cursor int, query string, typing bool, screenWidth, screenHeight int) string {
	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	branch := panel.Foreground(colors.Accent)
	muted := panel.Foreground(colors.TextMuted)

	rows := make([]string, len(worktrees))
	for index, worktree := range worktrees {
		rows[index] = panel.Render(filepath.Base(worktree.Path)+"  ") + branch.Render(worktree.Branch)
		if worktree.Current {
			rows[index] += panel.Render(" " + text.Text("worktrees.current", nil))
		}
		rows[index] += muted.Render("  " + worktree.Path)
	}

	return list{
		title:  text.Text("worktrees.title", nil),
		close:  closeHint(colors, keys, text, keymap.ListClose),
		width:  90,
		rows:   rows,
		cursor: cursor,
		hint:   windowHint(colors, keys, text, keymap.WorktreesSwitch, keymap.ListSearch),
	}.withSearch(colors, text, keys, query, typing, text.Text("list.noMatches", nil)).view(colors, screenWidth, screenHeight)
}
