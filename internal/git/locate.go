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
	// Operation незавершённая операция: merge, rebase, cherry-pick, revert
	// (#243), пусто без неё
	Operation string
}

// Locate называет ветку и worktree каталога dir. symbolic-ref знает ветку и в
// репозитории без коммитов, а rev-parse нужен только для detached HEAD. Префикс
// refs/heads/ снимается вручную: --short при теге с именем ветки отдаёт
// неоднозначное heads/v1. Хэш режется из полного той же shortHash, что у
// Worktrees: --short=7 при коллизии префикса отдаёт 8 символов и больше, и
// core.abbrev длину тоже меняет.
func Locate(dir string) (Location, error) {
	root, err := topLevel(dir)
	if err != nil {
		return Location{}, failed("rev-parse", err)
	}

	branch, err := run(call{dir: root}, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		var head string
		if head, err = run(call{dir: root}, "rev-parse", "HEAD"); err != nil {
			return Location{}, failed("rev-parse", err)
		}
		branch = shortHash(strings.TrimSpace(head))
	}

	return Location{Branch: strings.TrimPrefix(strings.TrimSpace(branch), "refs/heads/"), Worktree: filepath.Base(root), Operation: operation(root)}, nil
}

// Root это корень репозитория, в котором лежит dir: от него идут пути диффа.
func Root(dir string) (string, error) {
	root, err := topLevel(dir)
	return root, failed("rev-parse", err)
}

// shortHash это 7 первых символов полного хэша: так хэш detached HEAD виден в
// подвале и в окне worktree.
func shortHash(full string) string {
	return full[:min(len(full), 7)]
}
