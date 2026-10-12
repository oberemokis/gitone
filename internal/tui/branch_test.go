package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// B открывает список веток с текущей, k ставит курсор на другую, enter
// переключает её.
func TestBranchListSwitches(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "feature")
	write(t, repo, "a.txt", "two\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'B', Text: "B"})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "feature") {
		t.Fatalf("список веток не открылся:\n%s", body)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settle(model, cmd, 4)

	if got := runGit(t, repo, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature" {
		t.Fatalf("текущая ветка %q, ожидалась feature", got)
	}
}
