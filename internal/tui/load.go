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

// reloadMsg несёт итог перечитывания: новый дифф или ошибку git.
type reloadMsg struct {
	files []diff.File
	err   error
}

// reload читает дифф в команде, а не в Update, чтобы модель оставалась без
// ввода-вывода.
func reload(dir string, args []string) tea.Cmd {
	return func() tea.Msg {
		files, err := Load(dir, args)
		return reloadMsg{files: files, err: err}
	}
}
