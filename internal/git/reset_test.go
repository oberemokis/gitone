package git_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

// resetRepo это ветка из двух коммитов: a.txt «one», затем «two», и правка
// b.txt в индексе поверх.
func resetRepo(t *testing.T) (repo, first string) {
	t.Helper()

	repo = newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	first = revision(t, repo, "HEAD")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	write(t, repo, "b.txt", "staged\n")
	runGit(t, repo, "add", "b.txt")
	return repo, first
}

// Soft оставляет правку коммита и прежний индекс в индексе, mixed переносит их
// в рабочее дерево, hard стирает (#220, случаи 1-4).
func TestResetModes(t *testing.T) {
	cases := []struct {
		mode   git.ResetMode
		staged string
		a      string
	}{
		{mode: git.ResetSoft, staged: "a.txt\nb.txt", a: "two\n"},
		{mode: git.ResetMixed, staged: "", a: "two\n"},
		{mode: git.ResetHard, staged: "", a: "one\n"},
	}

	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			repo, first := resetRepo(t)

			published, err := git.Reset(repo, first, tc.mode)
			if err != nil || published {
				t.Fatalf("Reset = %v, %v; ожидался успех без upstream", published, err)
			}
			if got := revision(t, repo, "HEAD"); got != first {
				t.Fatalf("HEAD %s, ожидался %s", got, first)
			}
			if got := strings.TrimSpace(runGit(t, repo, "diff", "--cached", "--name-only")); got != tc.staged {
				t.Errorf("индекс %q, ожидался %q", got, tc.staged)
			}
			if got := readFile(t, filepath.Join(repo, "a.txt")); got != tc.a {
				t.Errorf("a.txt = %q, ожидалось %q", got, tc.a)
			}
		})
	}
}

// Цель текущей ветки это её upstream, другой ветки она сама; число снимаемых
// коммитов считается от HEAD. Без upstream текущая ветка отвергается (#231,
// случаи 1, 2 и 4).
func TestResetTarget(t *testing.T) {
	_, local := remotePair(t)
	runGit(t, local, "branch", "other")
	write(t, local, "b.txt", "local\n")
	commitAll(t, local)

	target, drops, err := git.ResetTarget(local, "main", true)
	if err != nil || target != "origin/main" || drops != 1 {
		t.Fatalf("ResetTarget(main) = %q, %d, %v; ожидались origin/main и 1", target, drops, err)
	}
	if target, drops, err = git.ResetTarget(local, "other", false); err != nil || target != "other" || drops != 1 {
		t.Fatalf("ResetTarget(other) = %q, %d, %v; ожидались other и 1", target, drops, err)
	}

	runGit(t, local, "switch", "-q", "other")
	_, _, err = git.ResetTarget(local, "other", true)
	refusedWith(t, err, "git.noUpstream")
}

// Reset под опубликованный коммит говорит, что ветка разошлась с upstream;
// reset неопубликованного коммита нет (случай 5).
func TestResetReportsPublishedCommits(t *testing.T) {
	_, local := remotePair(t)
	pushed := revision(t, local, "HEAD")
	write(t, local, "b.txt", "local\n")
	commitAll(t, local)

	published, err := git.Reset(local, pushed, git.ResetMixed)
	if err != nil || published {
		t.Fatalf("Reset неопубликованного = %v, %v; ожидался успех без расхождения", published, err)
	}

	runGit(t, local, "commit", "-q", "--allow-empty", "-m", "second")
	runGit(t, local, "push", "-q")
	published, err = git.Reset(local, pushed, git.ResetSoft)
	if err != nil || !published {
		t.Fatalf("Reset опубликованного = %v, %v; ожидалось расхождение с upstream", published, err)
	}
}
