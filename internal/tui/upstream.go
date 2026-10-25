package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// remoteBranchesMsg несёт удалённые ветки для выбора upstream ветки branch или
// ошибку git.
type remoteBranchesMsg struct {
	branch  string
	remotes []string
	err     error
}

func (msg remoteBranchesMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.chooseUpstream(msg), nil }

func remoteBranchesCmd(repo git.Repo, branch string) tea.Cmd {
	return func() tea.Msg {
		remotes, err := repo.RemoteBranches()
		return remoteBranchesMsg{branch: branch, remotes: remotes, err: err}
	}
}

// chooseUpstream открывает меню удалённых веток поверх окна веток (#230), как
// u в lazygit: ввода вручную нет, выбор только из того, что есть в локальных
// ссылках. Список устаревает без fetch, поэтому пустой список подсказывает
// его клавишу. Ответ, пришедший после закрытия окна, отбрасывается.
func (m Model) chooseUpstream(msg remoteBranchesMsg) Model {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case m.active != branchWindow:
		return m
	case len(msg.remotes) == 0:
		return m.putNotice(m.text.Text("upstream.none", i18n.Args{"key": firstKey(m.keys, keymap.GitFetch)}))
	}

	items := make([]menuItem, len(msg.remotes))
	for index, remote := range msg.remotes {
		items[index] = menuItem{label: visible(remote), run: func(m Model) (tea.Model, tea.Cmd) {
			note := m.text.Text("upstream.set", i18n.Args{"branch": visible(msg.branch), "upstream": visible(remote)})
			return m, setUpstreamCmd(m.repo, msg.branch, remote, note)
		}}
	}
	return m.openMenu(m.text.Text("upstream.title", i18n.Args{"branch": visible(msg.branch)}), items)
}

// setUpstreamCmd привязывает ветку к upstream и отвечает новым списком веток:
// окно веток остаётся открытым, тост говорит итог.
func setUpstreamCmd(repo git.Repo, branch, upstream, note string) tea.Cmd {
	return func() tea.Msg {
		msg := thenBranches(repo, func() error { return repo.SetUpstream(branch, upstream) })().(branchesMsg)
		if msg.err == nil {
			msg.note = note
		}
		return msg
	}
}
