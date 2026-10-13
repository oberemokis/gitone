package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// z уносит правку в stash, Z показывает список, a применяет его обратно.
func TestStashFromApp(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	model = settle(model, cmd, 4)
	if got := readText(t, repo+"/a.txt"); got != "one\n" {
		t.Fatalf("после z файл %q, ожидался возврат к HEAD", got)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "stash") {
		t.Fatalf("список stash не открылся:\n%s", body)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	settle(model, cmd, 4)
	if got := readText(t, repo+"/a.txt"); got != "two\n" {
		t.Fatalf("после a файл %q, ожидалась правка", got)
	}
}

// Stash это коммит слияния HEAD и индекса, и git diff stash@{0}^! отдаёт для
// него комбинированный diff --cc одной сырой записью. Дифф берётся как у
// git stash show -p: правка вне индекса и правка в индексе идут обычными
// файлами.
func TestStashDiffShowsWorktreeAndIndexFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	write(t, repo, "s.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	write(t, repo, "s.txt", "two\n")
	runGit(t, repo, "add", "s.txt")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	screen := screenOf(model)
	if got := openedFile(screen); got != "a.txt" {
		t.Fatalf("первый файл диффа stash %q, ожидался a.txt:\n%s", got, strings.Join(screen, "\n"))
	}
	if body := strings.Join(screen, "\n"); strings.Contains(body, "diff --cc") || !strings.Contains(body, "two") {
		t.Fatalf("дифф a.txt не обычный:\n%s", body)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	screen = screenOf(model)
	if got := openedFile(screen); got != "s.txt" {
		t.Fatalf("второй файл диффа stash %q, ожидался s.txt:\n%s", got, strings.Join(screen, "\n"))
	}
}

// Внешний git stash push сдвигает stash@{0}: открытый дифф держит свой stash
// по хэшу и при перечитывании не переходит на новый.
func TestStashDiffSurvivesExternalPush(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	write(t, repo, "a.txt", "three\n")
	runGit(t, repo, "stash", "push", "-q")
	model = pressReload(t, model)

	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "two") || strings.Contains(body, "three") {
		t.Fatalf("после перечитывания на экране не тот stash:\n%s", body)
	}
}

// d спрашивает, удаляет stash только y.
func TestStashDropNeedsYes(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	model = settle(model, cmd, 4)
	if out := runGit(t, repo, "stash", "list"); out == "" {
		t.Fatal("stash удалён без подтверждения")
	}

	// ответ «нет» оставил список открытым, вопрос задаётся в нём же
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	settle(model, cmd, 4)
	if out := runGit(t, repo, "stash", "list"); out != "" {
		t.Fatalf("stash не удалён после y: %q", out)
	}
}
