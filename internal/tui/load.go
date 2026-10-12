package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/diff"
	"gitone/internal/git"
)

// Load читает дифф каталога: cmd/gitone при запуске, а по r reload одним и тем
// же путём.
func Load(dir string, args []string) ([]diff.File, error) {
	text, err := git.Diff(dir, args)
	if err != nil {
		return nil, err
	}

	return diff.Parse(text), nil
}

// Staged возвращает пути, у которых индекс отличается от HEAD. Отметка в панели
// файлов берётся отсюда.
func Staged(dir string) (map[string]bool, error) {
	paths, err := git.StagedPaths(dir)
	if err != nil {
		return nil, err
	}

	staged := make(map[string]bool, len(paths))
	for _, path := range paths {
		staged[path] = true
	}

	return staged, nil
}

// Locate называет ветку и worktree для подвала.
func Locate(dir string) (git.Location, error) {
	return git.Locate(dir)
}

// reloadMsg несёт итог перечитывания: новый дифф, индекс, ветку или ошибку git.
type reloadMsg struct {
	files    []diff.File
	staged   map[string]bool
	location git.Location
	err      error
}

// reload читает дифф и индекс в команде, а не в Update, чтобы модель оставалась
// без ввода-вывода.
func reload(dir string, args []string) tea.Cmd {
	return func() tea.Msg {
		return readDiff(dir, args)
	}
}

// readDiff читает дифф, индекс и ветку: по r и при опросе одним путём. Ветку
// могли переключить в другом терминале, поэтому она читается вместе с диффом.
func readDiff(dir string, args []string) reloadMsg {
	files, err := Load(dir, args)
	if err != nil {
		return reloadMsg{err: err}
	}

	staged, err := Staged(dir)
	if err != nil {
		return reloadMsg{err: err}
	}

	location, err := Locate(dir)
	return reloadMsg{files: files, staged: staged, location: location, err: err}
}

// reloaded ставит перечитанный дифф на экран, а ошибку git в подвал. Если за
// время загрузки gitone сам что-то поменял, загрузка идёт ещё раз.
func (m Model) reloaded(msg reloadMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	// ponytail: окно поиска, открытое во время загрузки, показывает список
	// до перечитывания, выбор при этом ищется по имени в новом. Открыть
	// окно заново обновит список.
	if msg.err != nil {
		m = m.say(m.text.Error(msg.err))
	} else {
		m = m.applyReload(msg.files)
		m.staged, m.location = msg.staged, msg.location
	}

	if m.stale {
		return m.startReload()
	}
	return m, nil
}
