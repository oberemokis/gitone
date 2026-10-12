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

// reloadMsg несёт итог перечитывания: новый дифф, индекс или ошибку git.
type reloadMsg struct {
	files  []diff.File
	staged map[string]bool
	err    error
}

// reload читает дифф и индекс в команде, а не в Update, чтобы модель оставалась
// без ввода-вывода.
func reload(dir string, args []string) tea.Cmd {
	return func() tea.Msg {
		files, err := Load(dir, args)
		if err != nil {
			return reloadMsg{err: err}
		}

		staged, err := Staged(dir)
		return reloadMsg{files: files, staged: staged, err: err}
	}
}
