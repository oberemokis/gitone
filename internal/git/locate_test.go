package git_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

func TestLocate(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, repo string) string
		want    func(repo string) git.Location
	}{
		{
			name:    "ветка и каталог репозитория",
			prepare: func(t *testing.T, repo string) string { return repo },
			want:    func(repo string) git.Location { return git.Location{Branch: "main", Worktree: filepath.Base(repo)} },
		},
		{
			name: "из подкаталога тот же worktree",
			prepare: func(t *testing.T, repo string) string {
				write(t, repo, "sub/b.txt", "b\n")
				return filepath.Join(repo, "sub")
			},
			want: func(repo string) git.Location { return git.Location{Branch: "main", Worktree: filepath.Base(repo)} },
		},
		{
			name: "второй worktree",
			prepare: func(t *testing.T, repo string) string {
				other := filepath.Join(t.TempDir(), "side-tree")
				runGit(t, repo, "worktree", "add", "-q", "-b", "side", other)
				return other
			},
			want: func(string) git.Location { return git.Location{Branch: "side", Worktree: "side-tree"} },
		},
		{
			name: "тег с именем ветки",
			prepare: func(t *testing.T, repo string) string {
				runGit(t, repo, "switch", "-q", "-c", "v1")
				runGit(t, repo, "tag", "v1")
				return repo
			},
			want: func(repo string) git.Location { return git.Location{Branch: "v1", Worktree: filepath.Base(repo)} },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\n")
			commitAll(t, repo)

			got, err := git.Locate(tc.prepare(t, repo))
			if err != nil {
				t.Fatalf("Locate: %v", err)
			}
			if want := tc.want(repo); got != want {
				t.Fatalf("Locate = %+v, want %+v", got, want)
			}
		})
	}
}

// При detached HEAD вместо ветки короткий хэш коммита.
func TestLocateDetachedShowsShortHash(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	hash := strings.TrimSpace(runGit(t, repo, "rev-parse", "--short", "HEAD"))
	runGit(t, repo, "switch", "-q", "--detach")

	got, err := git.Locate(repo)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got.Branch != hash {
		t.Fatalf("Branch = %q, want короткий хэш %q", got.Branch, hash)
	}
}

// Длина хэша не зависит от core.abbrev: подвал (Locate) и окно worktree
// (Worktrees) показывают для detached HEAD одни и те же 7 символов.
func TestLocateDetachedHashMatchesWorktreesDespiteAbbrev(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "config", "core.abbrev", "12")
	hash := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	runGit(t, repo, "switch", "-q", "--detach")

	got, err := git.Locate(repo)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got.Branch != hash[:7] {
		t.Fatalf("Locate.Branch = %q, want %q", got.Branch, hash[:7])
	}

	worktrees, err := git.Worktrees(repo)
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(worktrees) != 1 || worktrees[0].Branch != hash[:7] {
		t.Fatalf("Worktrees = %+v, want одно дерево с веткой %q", worktrees, hash[:7])
	}
}

// В репозитории без коммитов ветка уже названа, хэша ещё нет.
func TestLocateUnbornBranch(t *testing.T) {
	repo := newRepo(t)

	got, err := git.Locate(repo)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got.Branch != "main" {
		t.Fatalf("Branch = %q, want main", got.Branch)
	}
}
