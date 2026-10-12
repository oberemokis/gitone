package git_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

// worktreeNames это имена каталогов списка с веткой и отметкой текущего.
func worktreeNames(worktrees []git.Worktree) []string {
	var names []string
	for _, worktree := range worktrees {
		name := filepath.Base(worktree.Path) + " " + worktree.Branch
		if worktree.Current {
			name += " *"
		}
		names = append(names, name)
	}
	return names
}

func TestWorktrees(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	side := filepath.Join(t.TempDir(), "side-tree")
	runGit(t, repo, "worktree", "add", "-q", "-b", "side", side)
	detached := filepath.Join(t.TempDir(), "loose-tree")
	runGit(t, repo, "worktree", "add", "-q", "--detach", detached)
	hash := strings.TrimSpace(runGit(t, repo, "rev-parse", "--short", "HEAD"))
	// каталог удалён с диска: git помечает его prunable
	gone := filepath.Join(t.TempDir(), "gone-tree")
	runGit(t, repo, "worktree", "add", "-q", "-b", "gone", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		want []string
	}{
		{name: "из основного", dir: repo, want: []string{filepath.Base(repo) + " main *", "side-tree side", "loose-tree " + hash}},
		{name: "из соседнего", dir: side, want: []string{filepath.Base(repo) + " main", "side-tree side *", "loose-tree " + hash}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			worktrees, err := git.Worktrees(tc.dir)
			if err != nil {
				t.Fatalf("Worktrees: %v", err)
			}
			if got := worktreeNames(worktrees); strings.Join(got, ", ") != strings.Join(tc.want, ", ") {
				t.Fatalf("Worktrees = %q, want %q", got, tc.want)
			}
		})
	}
}
