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
