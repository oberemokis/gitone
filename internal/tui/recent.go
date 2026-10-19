package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/config"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// recentRepo это строка окна недавних репозиториев (#248): путь корня,
// пометка, что каталога больше нет или он уже не репозиторий, и что он
// открыт сейчас.
type recentRepo struct {
	path    string
	missing bool
	current bool
}

// recentMsg несёт недавние репозитории из state.json.
type recentMsg struct {
	repos []recentRepo
}

func (msg recentMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.recentArrived(msg), nil }

// recentCmd читает список и проверяет каждый каталог: пропавший помечается,
// но из списка сам не убирается, диск мог быть просто не подключён.
func recentCmd(stateDir, dir string) tea.Cmd {
	return func() tea.Msg {
		current, _ := git.Root(dir)

		var repos []recentRepo
		for _, path := range config.ReadState(stateDir).RecentRepos {
			_, err := git.Root(path)
			repos = append(repos, recentRepo{path: path, missing: err != nil, current: path == current})
		}
		return recentMsg{repos: repos}
	}
}

// rememberRepoCmd ставит репозиторий каталога dir первым в state.json. Без
// каталога состояния (тесты) или вне репозитория записывать нечего. Ошибка
// записи интерфейс не останавливает: список недавних это удобство.
func rememberRepoCmd(stateDir, dir string) tea.Cmd {
	if stateDir == "" {
		return nil
	}
	return func() tea.Msg {
		if root, err := git.Root(dir); err == nil {
			_ = config.RememberRepo(stateDir, root)
		}
		return nil
	}
}

// recentArrived открывает окно недавних репозиториев.
func (m Model) recentArrived(msg recentMsg) Model {
	return m.arrived(listArrival{shown: recentWindow, set: func(m *Model) { m.recent = msg.repos }})
}

// recentActions это действия окна недавних: recent.open открывает выбранный
// репозиторий тем же путём, что переключение worktree. Пропавший каталог не
// открывается, подвал говорит почему.
var recentActions = []windowAction[recentRepo]{
	{action: keymap.RecentOpen, changesRepo: true, run: func(m Model, repo recentRepo) (tea.Model, tea.Cmd) {
		switch {
		case repo.missing:
			return m.putNotice(m.text.Text("recent.missing", i18n.Args{"path": visible(repo.path)}), true), nil
		case repo.current:
			m.active = noWindow
			return m, nil
		}
		m.active = noWindow
		return m, tea.Batch(checkWorktreeCmd(repo.path), rememberRepoCmd(m.stateDir, repo.path))
	}},
}

var recentHint = actionsOf(recentActions)

func (m Model) recentKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.recent)); handled {
		return next, cmd
	}
	return dispatch(m, recentActions, key.String(), m.recent)
}

// recentBox рисует пути недавних репозиториев: открытый сейчас и пропавший
// помечены словом.
func recentBox(p paint, repos []recentRepo, cursor int, screenWidth, screenHeight int) string {
	box := list{
		title:  p.text.Text("recent.title", nil),
		close:  p.close(keymap.ListClose),
		width:  90,
		cursor: cursor,
		empty:  []string{p.text.Text("recent.empty", nil)},
		hint:   p.hint(recentHint...),
	}

	panel, muted := p.panel(), p.muted()
	path := panel.Foreground(p.colors.Accent)
	gap := panel.Render("  ")
	width := box.rowWidth(screenWidth)

	box.rows = make([]string, len(repos))
	for index, repo := range repos {
		mark := ""
		switch {
		case repo.missing:
			mark = p.text.Text("recent.missingMark", nil)
		case repo.current:
			mark = p.text.Text("worktrees.current", nil)
		}
		markWidth := ansi.StringWidth(mark)
		box.rows[index] = tableRow(panel, []int{max(width-markWidth-columnGap, 1)}, []lipgloss.Style{path}, repo.path)
		if mark != "" {
			box.rows[index] += gap + tableRow(panel, []int{markWidth}, []lipgloss.Style{muted}, mark)
		}
	}

	return box.view(p, screenWidth, screenHeight)
}
