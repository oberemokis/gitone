package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// lostCommitRepo это репозиторий, где второй коммит «lost» снят reset --hard:
// в логе его нет, в reflog он есть. Окно reflog открыто, курсор на reset.
func lostCommitRepo(t *testing.T) (repo, lost string, model tea.Model) {
	t.Helper()

	repo = newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "lost line\n")
	runGit(t, repo, "commit", "-q", "-am", "lost")
	lost = runGit(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "reset", "-q", "--hard", "HEAD~1")

	model = resize(open(t, options(repo, layout.Focus())), 120, 24)
	return repo, lost, openAsync(model, key('L'))
}

// L открывает reflog: записи с хэшем, действием и описанием (#240, случай 1).
func TestReflogList(t *testing.T) {
	_, lost, model := lostCommitRepo(t)

	body := screenText(model)
	for _, want := range []string{"reset", "moving to HEAD~1", lost[:7], "commit", "lost"} {
		if !strings.Contains(body, want) {
			t.Fatalf("в окне нет %q:\n%s", want, body)
		}
	}
}

// enter на записи потерянного коммита показывает его дифф (случай 2).
func TestReflogOpenShowsCommit(t *testing.T) {
	_, _, model := lostCommitRepo(t)

	model, _ = model.Update(key('j'))
	model = enterKey(model)

	if body := screenText(model); strings.Contains(body, "Reflog") || !strings.Contains(body, "lost line") {
		t.Fatalf("enter не показал дифф потерянного коммита:\n%s", body)
	}
}

// space переходит на коммит записи без ветки, n ставит на него ветку (случай 3).
func TestReflogCheckoutAndBranch(t *testing.T) {
	repo, lost, model := lostCommitRepo(t)

	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('n'))
	enterKey(typeText(model, "rescue"))
	if got := runGit(t, repo, "rev-parse", "rescue"); got != lost {
		t.Fatalf("rescue на %s, ожидался потерянный коммит %s", got, lost)
	}

	repo, lost, model = lostCommitRepo(t)
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if footer := footerOf(model); !strings.Contains(footer, "detached") {
		t.Fatalf("подвал %q, ожидался переход без ветки", footer)
	}
	if got := runGit(t, repo, "rev-parse", "HEAD"); got != lost {
		t.Fatalf("HEAD на %s, ожидался потерянный коммит %s", got, lost)
	}
	if branch := runGit(t, repo, "branch", "--show-current"); branch != "" {
		t.Fatalf("HEAD на ветке %q, ожидался без ветки", branch)
	}
}
