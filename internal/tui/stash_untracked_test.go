package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Дифф stash, сделанного с неотслеженными файлами, показывает и их: git diff
// по самому stash их не видит, они в третьем родителе (#314, случай 1).
func TestStashDiffShowsUntracked(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	write(t, repo, "new.txt", "fresh\n")
	runGit(t, repo, "stash", "push", "-q", "-u")
	model := openAsync(resize(open(t, options(repo, layout.Default())), 120, 24), key('Z'))

	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if body := screenText(toggleFiles(model)); !strings.Contains(body, "new.txt") || !strings.Contains(body, "a.txt") {
		t.Fatalf("в диффе stash нет неотслеженного new.txt:\n%s", body)
	}
}
