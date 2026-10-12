package tui_test

import (
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
)

// gitOut запускает git в каталоге и возвращает вывод без пробелов по краям.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func typeKeys(model tea.Model, text string) tea.Model {
	for _, char := range text {
		model, _ = model.Update(tea.KeyPressMsg{Code: char, Text: string(char)})
	}
	return model
}

// c открывает поле темы, набранное уходит в коммит по enter.
func TestCommitFromApp(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	model := openStage(t, repo)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model = typeKeys(model, "fix text")
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	if body := strings.Join(screenOf(model), "\n"); strings.Contains(body, "> fix") {
		t.Fatalf("поле не закрылось после коммита:\n%s", body)
	}
	if subject, err := git.StagedPaths(repo); err != nil || len(subject) != 0 {
		t.Fatalf("индекс не пуст после коммита: %q, %v", subject, err)
	}
	if got := gitOut(t, repo, "log", "-1", "--format=%s"); got != "fix text" {
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
	before := gitOut(t, repo, "rev-parse", "HEAD")

	model := openStage(t, repo)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model = typeKeys(model, "черновик")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if strings.Contains(strings.Join(screenOf(model), "\n"), "> черновик") {
		t.Fatal("поле не закрылось по esc")
	}
	if after := gitOut(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatal("коммит прошёл после esc")
	}
}

// q и ? в поле темы набираются текстом: выход из приложения там не срабатывает.
func TestQuitKeysTypeIntoSubject(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := openStage(t, repo)
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
