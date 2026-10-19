package tui_test

import (
	"os/exec"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Файл с конфликтом слияния числится в индексе неслитым. Пробел после правки
// маркеров помечает его разрешённым: Unstage стёр бы стадии конфликта, и коммит
// слияния записал бы «ours».
func TestSpaceResolvesMergeConflict(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "base\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", "topic\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "main\n")
	commitAll(t, repo)
	merge := exec.Command("git", "merge", "topic")
	merge.Dir = repo
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("слияние прошло без конфликта:\n%s", out)
	}
	write(t, repo, "a.txt", "merged\n")

	model := toggleFiles(resize(open(t, options(repo, layout.Default())), 100, 20))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	settle(model, cmd, 4)

	if unmerged := runGit(t, repo, "ls-files", "--unmerged"); unmerged != "" {
		t.Fatalf("стадии конфликта остались:\n%s", unmerged)
	}
	if got := runGit(t, repo, "show", ":a.txt"); got != "merged" {
		t.Fatalf("в индексе %q, ожидалась разрешённая правка merged", got)
	}
}
