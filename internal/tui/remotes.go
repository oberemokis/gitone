package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// remotesMsg несёт список remote или ошибку git. note это итог правки, после
// которой список пришёл в открытое окно.
type remotesMsg struct {
	repo    git.Repo
	remotes []git.Remote
	err     error
	note    string
}

func (msg remotesMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.remotesArrived(msg), nil }
func (msg remotesMsg) from() git.Repo                     { return msg.repo }

func remotesCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		remotes, err := repo.Remotes()
		return remotesMsg{repo: repo, remotes: remotes, err: err}
	}
}

// thenRemotes выполняет правку remote и отвечает новым списком с итогом note,
// а отказ git ошибкой: окно остаётся открытым.
func thenRemotes(repo git.Repo, note string, change func() error) tea.Cmd {
	return func() tea.Msg {
		if err := change(); err != nil {
			return remotesMsg{repo: repo, err: err}
		}
		msg := remotesCmd(repo)().(remotesMsg)
		msg.note = note
		return msg
	}
}

// remotesArrived открывает окно remote или обновляет открытое (arrived), подвал
// говорит итог правки. Ответ правки это ответ с note.
func (m Model) remotesArrived(msg remotesMsg) Model {
	m = m.arrived(listArrival{shown: remoteWindow, err: msg.err, edit: msg.note != "", set: func(m *Model) { m.remotes = msg.remotes }})
	if msg.err == nil && msg.note != "" {
		m = m.sayCalm(msg.note)
	}
	return m
}

// remoteActions это действия окна remote (#233): remotes.new спрашивает имя и
// адрес, remotes.delete удаляет после вопроса. Пока идёт pull, push или fetch,
// правки отказывают: git работает с тем же remote.
var remoteActions = []windowAction[git.Remote]{
	{action: keymap.RemotesNew, anyRow: true, changesRepo: true, run: func(m Model, _ git.Remote) (tea.Model, tea.Cmd) {
		return m.askRemote(), nil
	}},
	{action: keymap.RemotesDelete, changesRepo: true, run: func(m Model, remote git.Remote) (tea.Model, tea.Cmd) {
		return m.askRemoveRemote(remote), nil
	}},
}

var remoteHint = actionsOf(remoteActions)

func (m Model) remoteKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.remotes)); handled {
		return next, cmd
	}
	return dispatch(m, remoteActions, key.String(), m.remotes)
}

// askRemote спрашивает имя нового remote, потом адрес: адрес вводится руками,
// угадывать его по адресу origin gitone не берётся.
func (m Model) askRemote() Model {
	return m.openPrompt(m.text.Text("remotes.newTitle", nil), "", func(m Model, name string) (tea.Model, tea.Cmd) {
		title := m.text.Text("remotes.urlTitle", i18n.Args{"name": visible(name)})
		return m.openPrompt(title, "", func(m Model, url string) (tea.Model, tea.Cmd) {
			note := m.text.Text("remotes.added", i18n.Args{"name": visible(name)})
			return m, thenRemotes(m.repo, note, func() error { return m.repo.AddRemote(name, url) })
		}), nil
	})
}

// askRemoveRemote спрашивает, удалить ли remote. На remote с upstream текущей
// ветки вопрос предупреждает: upstream пропадёт вместе с ним.
func (m Model) askRemoveRemote(remote git.Remote) Model {
	args := questionArgs(m.keys)
	args["name"] = visible(remote.Name)
	question := "remotes.deleteQuestion"
	if remote.Upstream {
		question = "remotes.deleteUpstreamQuestion"
	}

	note := m.text.Text("remotes.deleted", i18n.Args{"name": visible(remote.Name)})
	return m.ask(m.text.Text(question, args), func(m Model) (tea.Model, tea.Cmd) {
		return m, thenRemotes(m.repo, note, func() error { return m.repo.RemoveRemote(remote.Name) })
	})
}

// remoteBox рисует remote таблицей: имя цветом accent, адреса fetch и push.
// Remote с upstream текущей ветки помечен словом за именем.
func remoteBox(p paint, remotes []git.Remote, cursor int, question string, screenWidth, screenHeight int) string {
	box := list{
		title:    p.text.Text("remotes.title", nil),
		close:    p.close(keymap.ListClose),
		width:    100,
		cursor:   cursor,
		empty:    []string{p.text.Text("remotes.empty", nil)},
		hint:     p.hint(remoteHint...),
		question: question,
	}

	panel, muted := p.panel(), p.muted()
	upstream := p.text.Text("remotes.upstream", nil)

	titles := []string{p.text.Text("remotes.column.name", nil), p.text.Text("remotes.column.fetch", nil), p.text.Text("remotes.column.push", nil)}
	nameWidth := ansi.StringWidth(titles[0])
	for _, remote := range remotes {
		nameWidth = max(nameWidth, ansi.StringWidth(visible(remote.Name)+" "+upstream))
	}
	widths := []int{min(nameWidth, maxNameColumn), 0, 0}
	rest := max(box.rowWidth(screenWidth)-widths[0]-2*columnGap, 2)
	widths[1], widths[2] = rest/2, rest-rest/2

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted}, titles...)

	name := panel.Foreground(p.colors.Accent)
	gap := panel.Render("  ")
	box.rows = make([]string, len(remotes))
	for index, remote := range remotes {
		box.rows[index] = markedCell(panel, name, widths[0], remote.Name, remote.Upstream, upstream) + gap +
			tableRow(panel, widths[1:], []lipgloss.Style{panel, muted}, remote.Fetch, remote.Push)
	}

	return box.view(p, screenWidth, screenHeight)
}
