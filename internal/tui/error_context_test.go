package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/layout"
)

// Ошибка перечитывания в тосте называет операцию git один раз и несёт текст
// самого git.
func TestReloadErrorNamesOperation(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	notice := noticeOf(pressReload(t, model))

	if strings.Count(notice, "diff: ") != 1 || !strings.Contains(notice, "diff: fatal: not a git repository") {
		t.Fatalf("тост = %q, want «diff: » один раз и текст git", notice)
	}
}
