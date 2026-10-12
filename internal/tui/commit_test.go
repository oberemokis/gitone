package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/layout"
)

// c открывает поле темы, набранное уходит в коммит по enter.
func TestCommitFromApp(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model = typeText(model, "fix text")
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	if body := strings.Join(screenOf(model), "\n"); strings.Contains(body, "> fix") {
		t.Fatalf("поле не закрылось после коммита:\n%s", body)
	}
	if subject, err := git.StagedPaths(repo); err != nil || len(subject) != 0 {
		t.Fatalf("индекс не пуст после коммита: %q, %v", subject, err)
	}
	if got := runGit(t, repo, "log", "-1", "--format=%s"); got != "fix text" {
		t.Fatalf("тема коммита %q, ожидалась fix text", got)
	}
}

// esc закрывает поле без коммита.
func TestEscCancelsCommit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")
	before := runGit(t, repo, "rev-parse", "HEAD")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model = typeText(model, "черновик")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if strings.Contains(strings.Join(screenOf(model), "\n"), "> черновик") {
		t.Fatal("поле не закрылось по esc")
	}
	if after := runGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatal("коммит прошёл после esc")
	}
}

// q и ? в поле темы набираются текстом: выход из приложения там не срабатывает.
func TestQuitKeysTypeIntoSubject(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})

	if cmd != nil {
		if _, quits := cmd().(tea.QuitMsg); quits {
			t.Fatal("q выходит из приложения, хотя набирается в тему")
		}
	}
	if !strings.Contains(strings.Join(screenOf(model), "\n"), "> q") {
		t.Fatal("q не попал в поле темы")
	}
}
