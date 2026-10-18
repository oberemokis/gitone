package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
)

// remotesMsg несёт список remote или ошибку git. note это итог правки, после
// которой список пришёл в открытое окно.
type remotesMsg struct {
	remotes []git.Remote
	err     error
	note    string
}

func remotesCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		remotes, err := git.Remotes(dir)
		return remotesMsg{remotes: remotes, err: err}
	}
}

// thenRemotes выполняет правку remote и отвечает новым списком с итогом note,
// а отказ git ошибкой: окно остаётся открытым.
func thenRemotes(dir, note string, change func() error) tea.Cmd {
	return func() tea.Msg {
		if err := change(); err != nil {
			return remotesMsg{err: err}
		}
		msg := remotesCmd(dir)().(remotesMsg)
		msg.note = note
		return msg
	}
}

// remotesArrived открывает окно remote или обновляет открытое.
func (m Model) remotesArrived(msg remotesMsg) Model {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case m.active == noWindow:
		m.remotes, m.cursor, m.active = msg.remotes, 0, remoteWindow
	case m.active == remoteWindow:
		m.remotes = msg.remotes
		m = m.moveCursor(0, len(m.remotes))
		if msg.note != "" {
			m = m.sayCalm(msg.note)
		}
	}
	return m
}

// remoteKey отдаёт клавиши окну remote (#233): remotes.new спрашивает имя и
// адрес, remotes.delete удаляет после вопроса. Пока идёт pull, push или fetch,
// правки отказывают: git работает с тем же remote.
func (m Model) remoteKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.remotes)); handled {
		return next, cmd
	}

	pressed := key.String()
	switch {
	case m.keys.Is(keymap.RemotesNew, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.askRemote(), nil
	case m.keys.Is(keymap.RemotesDelete, pressed) && len(m.remotes) > 0:
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.askRemoveRemote(m.remotes[m.cursor]), nil
	}

	return m, nil
}

// askRemote спрашивает имя нового remote, потом адрес: адрес вводится руками,
// угадывать его по адресу origin gitone не берётся.
func (m Model) askRemote() Model {
	return m.openPrompt(m.text.Text("remotes.newTitle", nil), "", func(m Model, name string) (tea.Model, tea.Cmd) {
		title := m.text.Text("remotes.urlTitle", i18n.Args{"name": visible(name)})
		return m.openPrompt(title, "", func(m Model, url string) (tea.Model, tea.Cmd) {
			note := m.text.Text("remotes.added", i18n.Args{"name": visible(name)})
			return m, thenRemotes(m.dir, note, func() error { return git.AddRemote(m.dir, name, url) })
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
		return m, thenRemotes(m.dir, note, func() error { return git.RemoveRemote(m.dir, remote.Name) })
	})
}

// remoteBox рисует remote таблицей: имя цветом accent, адреса fetch и push.
// Remote с upstream текущей ветки помечен словом за именем.
func remoteBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, remotes []git.Remote, cursor int, question string, screenWidth, screenHeight int) string {
	box := list{
		title:    text.Text("remotes.title", nil),
		close:    closeHint(colors, keys, text, keymap.ListClose),
		width:    100,
		cursor:   cursor,
		empty:    []string{text.Text("remotes.empty", nil)},
		hint:     windowHint(colors, keys, text, keymap.RemotesNew, keymap.RemotesDelete),
		question: question,
	}

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)
	upstream := text.Text("remotes.upstream", nil)

	titles := []string{text.Text("remotes.column.name", nil), text.Text("remotes.column.fetch", nil), text.Text("remotes.column.push", nil)}
	nameWidth := ansi.StringWidth(titles[0])
	for _, remote := range remotes {
		nameWidth = max(nameWidth, ansi.StringWidth(visible(remote.Name)+" "+upstream))
	}
	widths := []int{min(nameWidth, maxNameColumn), 0, 0}
	rest := max(box.rowWidth(screenWidth)-widths[0]-2*columnGap, 2)
	widths[1], widths[2] = rest/2, rest-rest/2

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted}, titles...)

	name := panel.Foreground(colors.Accent)
	gap := panel.Render("  ")
	box.rows = make([]string, len(remotes))
	for index, remote := range remotes {
		box.rows[index] = markedCell(panel, name, widths[0], remote.Name, remote.Upstream, upstream) + gap +
			tableRow(panel, widths[1:], []lipgloss.Style{panel, muted}, remote.Fetch, remote.Push)
	}

	return box.view(colors, screenWidth, screenHeight)
}
