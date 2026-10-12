package tui

import (
	"gitone/internal/diff"
	"gitone/internal/git"
)

// Load читает дифф каталога: cmd/gitone при запуске, а позже обновление по r
// (#12) одним и тем же путём.
func Load(dir string, args []string) ([]diff.File, error) {
	text, err := git.Diff(dir, args)
	if err != nil {
		return nil, err
	}

	return diff.Parse(text), nil
}
