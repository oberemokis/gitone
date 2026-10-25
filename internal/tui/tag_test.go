package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// tagRepo это репозиторий с двумя коммитами: лёгкий тег light на первом и
// аннотированный release на втором, открытый окном тегов.
func tagRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	// окно идёт от новых тегов к старым, даты разведены, чтобы release был сверху
	repo := newRepo(t)
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-01T10:00:00Z")
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "tag", "light")
	t.Setenv("GIT_COMMITTER_DATE", "2026-02-01T10:00:00Z")
	write(t, repo, "b.txt", "two\n")
	commitAll(t, repo)
	runGit(t, repo, "tag", "-a", "release", "-m", "first release")

	model := resize(open(t, options(repo, layout.Focus())), 120, 24)
	return repo, openAsync(model, key('t'))
}

func objectType(t *testing.T, repo, tag string) string {
	t.Helper()
	return runGit(t, repo, "cat-file", "-t", "refs/tags/"+tag)
}

// t открывает окно: оба тега видны, аннотированный помечен и показывает своё
// сообщение (#238, случай 1).
func TestTagsListShowsBothKinds(t *testing.T) {
	_, model := tagRepo(t)

	body := screenText(model)
	if !strings.Contains(body, "release annotated") || !strings.Contains(body, "first release") {
		t.Fatalf("аннотированный тег не виден:\n%s", body)
	}
	if !strings.Contains(body, "light") || strings.Contains(body, "light annotated") {
		t.Fatalf("лёгкий тег не виден или помечен аннотированным:\n%s", body)
	}
}

// enter на теге первого коммита не показывает дифф до рабочего дерева, подвал
// объясняет почему; на обычном коммите дифф коммита (#297, случаи 1 и 2).
func TestTagsDiffOfFirstCommit(t *testing.T) {
	repo, _ := tagRepo(t)
	write(t, repo, "a.txt", "worktree\n")
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('t'))

	model, _ = model.Update(key('j'))
	model = enterKey(model)
	if footer := footerOf(model); !strings.Contains(footer, "the first commit is not shown") {
		t.Fatalf("подвал %q, ожидалось объяснение", footer)
	}
	// дифф от первого коммита до рабочего дерева показал бы и b.txt второго
	if body := screenText(toggleFiles(model)); strings.Contains(body, "b.txt") {
		t.Fatalf("показан дифф до рабочего дерева:\n%s", body)
	}

	model = openAsync(model, key('t'))
	model = enterKey(model)
	if body := screenText(model); !strings.Contains(body, "b.txt") || strings.Contains(body, "worktree") {
		t.Fatalf("дифф release не дифф его коммита:\n%s", body)
	}
}

// n спрашивает имя, потом сообщение: с сообщением тег аннотированный, пустое
// сообщение даёт лёгкий тег (случай 2).
func TestTagsNew(t *testing.T) {
	repo, model := tagRepo(t)

	model, _ = model.Update(key('n'))
	if body := screenText(model); !strings.Contains(body, "New tag on HEAD") {
		t.Fatalf("n не открыл поле имени:\n%s", body)
	}
	model = enterKey(typeText(model, "v2"))
	if body := screenText(model); !strings.Contains(body, "Message for v2") {
		t.Fatalf("enter не спросил сообщение:\n%s", body)
	}
	model = enterKey(typeText(model, "second"))

	if got := objectType(t, repo, "v2"); got != "tag" {
		t.Fatalf("v2 это %q, ожидался аннотированный тег", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Tag v2 created on HEAD") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}

	model, _ = model.Update(key('n'))
	enterKey(enterKey(typeText(model, "v3")))
	if got := objectType(t, repo, "v3"); got != "commit" {
		t.Fatalf("v3 это %q, ожидался лёгкий тег", got)
	}
}

// Тег с занятым именем git не создаёт, подвал говорит причину (случай 5).
func TestTagsNewTakenName(t *testing.T) {
	_, model := tagRepo(t)

	model, _ = model.Update(key('n'))
	model = enterKey(enterKey(typeText(model, "light")))

	if footer := footerOf(model); !strings.Contains(footer, "already exists") {
		t.Fatalf("подвал %q, ожидался отказ git", footer)
	}
}

// d удаляет тег только после y (случай 3).
func TestTagsDeleteNeedsYes(t *testing.T) {
	repo, model := tagRepo(t)

	model, _ = model.Update(key('d'))
	if body := screenText(model); !strings.Contains(body, `Delete tag "release" locally? y yes`) {
		t.Fatalf("окно не задало вопрос:\n%s", body)
	}
	model = pressAndSettle(model, key('n'))
	if got := runGit(t, repo, "tag", "-l"); got != "light\nrelease" {
		t.Fatalf("теги %q, удалено без y", got)
	}

	model, _ = model.Update(key('d'))
	model = answerYes(model)
	if got := runGit(t, repo, "tag", "-l"); got != "light" {
		t.Fatalf("теги %q, ожидался один light", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Tag release deleted") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

// P отправляет тег под курсором на origin, подвал называет его (случай 4).
func TestTagsPush(t *testing.T) {
	origin, repo := remoteRepo(t)
	runGit(t, repo, "tag", "v1")

	model := pressAndSettle(openAsync(openRemote(t, repo), key('t')), key('P'))

	if got, want := runGit(t, origin, "rev-parse", "v1"), runGit(t, repo, "rev-parse", "v1"); got != want {
		t.Fatalf("v1 на origin %q, ожидался %q", got, want)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Tag v1 pushed to origin") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}
