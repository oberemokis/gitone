package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// reflogMsg несёт reflog HEAD или ошибку git.
type reflogMsg struct {
	repo    git.Repo
	entries []git.ReflogEntry
	err     error
}

func (msg reflogMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.reflogArrived(msg), nil }
func (msg reflogMsg) from() git.Repo                     { return msg.repo }

func reflogCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		entries, err := repo.Reflog()
		return reflogMsg{repo: repo, entries: entries, err: err}
	}
}

// reflogArrived открывает окно reflog с курсором на последней записи.
func (m Model) reflogArrived(msg reflogMsg) Model {
	return m.arrived(listArrival{shown: reflogWindow, err: msg.err, set: func(m *Model) { m.reflog = msg.entries }})
}

// reflogActions это действия окна reflog (#240). Клавиши те же, что у лога:
// log.open показывает дифф коммита записи, log.checkout переходит на него без
// ветки, log.newBranch ставит на него ветку. Так потерянный коммит
// возвращается в историю.
var reflogActions = []windowAction[git.ReflogEntry]{
	{action: keymap.LogOpen, run: func(m Model, entry git.ReflogEntry) (tea.Model, tea.Cmd) {
		return m.showCommit(git.LogEntry{Hash: entry.Hash, Root: entry.Root})
	}},
	{action: keymap.LogCheckout, changesRepo: true, run: func(m Model, entry git.ReflogEntry) (tea.Model, tea.Cmd) {
		return m.changeHistory(m.text.Text("log.detached", i18n.Args{"hash": shortCommit(entry.Hash)}), func(repo git.Repo) error {
			return repo.CheckoutCommit(entry.Hash)
		})
	}},
	{action: keymap.LogNewBranch, changesRepo: true, run: func(m Model, entry git.ReflogEntry) (tea.Model, tea.Cmd) {
		return m.askBranchName(shortCommit(entry.Hash), entry.Hash), nil
	}},
}

var reflogHint = actionsOf(reflogActions, keymap.ListSearch)

func (m Model) reflogKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.reflogShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}
	return dispatch(m, reflogActions, key.String(), shown)
}

// reflogShown это записи под фильтром поиска по хэшу, действию и описанию.
func (m Model) reflogShown() []git.ReflogEntry {
	return matching(m.reflog, m.query, func(entry git.ReflogEntry) string {
		return entry.Hash + "\x00" + entry.Action + "\x00" + entry.Subject
	})
}

// reflogBox рисует reflog таблицей, как лог: хэш цветом accent, действие,
// описание и время записи приглушённое.
func reflogBox(p paint, entries []git.ReflogEntry, cursor int, query string, typing bool, screenWidth, screenHeight int) string {
	box := list{
		title:  p.text.Text("reflog.title", nil),
		close:  p.close(keymap.ListClose),
		cursor: cursor,
		empty:  []string{p.text.Text("reflog.empty", nil)},
		hint:   p.hint(reflogHint...),
		fill:   true,
	}.withSearch(p, query, typing, p.text.Text("list.noMatches", nil))

	panel, muted := p.panel(), p.muted()

	titles := []string{p.text.Text("log.column.commit", nil), p.text.Text("reflog.column.action", nil), p.text.Text("log.column.message", nil), p.text.Text("log.column.date", nil)}
	actionWidth, dateWidth := ansi.StringWidth(titles[1]), ansi.StringWidth(titles[3])
	for _, entry := range entries {
		actionWidth = max(actionWidth, ansi.StringWidth(visible(entry.Action)))
		dateWidth = max(dateWidth, ansi.StringWidth(visible(entry.When)))
	}
	widths := []int{hashColumn, min(actionWidth, maxNameColumn), 0, min(dateWidth, maxDateColumn)}
	widths[2] = max(box.rowWidth(screenWidth)-widths[0]-widths[1]-widths[3]-3*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted, muted}, titles...)

	styles := []lipgloss.Style{panel.Foreground(p.colors.Accent), muted, panel, muted}
	box.rows = make([]string, len(entries))
	for index, entry := range entries {
		box.rows[index] = tableRow(panel, widths, styles, shortCommit(entry.Hash), entry.Action, entry.Subject, entry.When)
	}

	return box.view(p, screenWidth, screenHeight)
}
