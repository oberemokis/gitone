package git

import (
	"path/filepath"
	"strings"
)

// Location это ветка и worktree, на которых стоит рабочая копия. Branch при
// detached HEAD это короткий хэш коммита, Worktree имя каталога его корня.
type Location struct {
	Branch   string
	Worktree string
}

// Locate называет ветку и worktree каталога dir. symbolic-ref знает ветку и в
// репозитории без коммитов, а rev-parse нужен только для detached HEAD.
func Locate(dir string) (Location, error) {
	root, err := topLevel(dir)
	if err != nil {
		return Location{}, failed("rev-parse", err)
	}

	branch, err := run(call{dir: root}, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		branch, err = run(call{dir: root}, "rev-parse", "--short", "HEAD")
	}
	if err != nil {
		return Location{}, failed("rev-parse", err)
	}

	return Location{Branch: strings.TrimSpace(branch), Worktree: filepath.Base(root)}, nil
}
