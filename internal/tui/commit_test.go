package tui_test

import (
	"os"
	"path/filepath"
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

// commitFooter набирает тему в поле коммита, отправляет по enter и возвращает
// подвал после ответа git.
func commitFooter(model tea.Model) string {
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model = typeText(model, "x")
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	screen := screenOf(settle(model, cmd, 4))

	return screen[footerRow(screen)]
}

// Без застейдженных правок подвал называет причину текстом из словаря. Отчёт
// git commit («On branch main») причины первой строкой не даёт.
func TestCommitWithNothingStagedExplainsInFooter(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	opts := options(repo, layout.Default())
	opts.Text = pseudo()

	footer := commitFooter(resize(open(t, opts), 100, 20))

	if !strings.Contains(footer, "commit: ⟦nothing to commit⟧") {
		t.Fatalf("подвал = %q, ожидалось «commit: ⟦nothing to commit⟧»", footer)
	}
}

// Хук без вывода отказывает, и подвал называет код выхода: «commit: » с пустым
// текстом ничего не говорил.
func TestCommitSilentHookShowsExitStatus(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")
	write(t, repo, ".git/hooks/pre-commit", "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(filepath.Join(repo, ".git/hooks/pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}

	footer := commitFooter(resize(open(t, options(repo, layout.Default())), 100, 20))

	if !strings.Contains(footer, "commit: exit status 1") {
		t.Fatalf("подвал = %q, ожидалось «commit: exit status 1»", footer)
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
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = settle(model, cmd, 4)

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
