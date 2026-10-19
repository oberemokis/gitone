package git

import "strings"

// Worktree это рабочее дерево репозитория. Branch при detached HEAD это
// короткий хэш, Current отмечает дерево, в котором стоит каталог запроса.
type Worktree struct {
	Path    string
	Branch  string
	Current bool
}

// Worktrees перечисляет рабочие деревья репозитория в порядке git: основное
// первым.
func (r Repo) Worktrees() ([]Worktree, error) {
	out, err := r.inRoot("worktree", "list", "--porcelain")
	if err != nil {
		return nil, failed("worktree", err)
	}

	var worktrees []Worktree
	for _, block := range strings.Split(strings.TrimSpace(out), "\n\n") {
		if worktree, found := parseWorktree(block); found {
			worktree.Current = worktree.Path == r.root
			worktrees = append(worktrees, worktree)
		}
	}

	return worktrees, nil
}

// parseWorktree разбирает запись git worktree list --porcelain. Голое
// хранилище и дерево, удалённое с диска (prunable), в список не идут:
// переключаться на них некуда.
func parseWorktree(block string) (Worktree, bool) {
	var worktree Worktree
	for _, line := range strings.Split(block, "\n") {
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			worktree.Path = value
		case "HEAD":
			// строка ветки идёт после HEAD и заменит хэш
			worktree.Branch = shortHash(value)
		case "branch":
			worktree.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare", "prunable":
			return Worktree{}, false
		}
	}

	return worktree, worktree.Path != ""
}
