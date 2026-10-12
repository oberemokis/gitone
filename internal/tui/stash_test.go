package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// z уносит правку в stash, Z показывает список, a применяет его обратно.
func TestStashFromApp(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := openStage(t, repo)
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
	model = settle(model, cmd, 4)
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

	model := openStage(t, repo)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	model = settle(model, cmd, 4)
	if out := gitOut(t, repo, "stash", "list"); out == "" {
		t.Fatal("stash удалён без подтверждения")
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	model = settle(model, cmd, 4)
	_ = model
	if out := gitOut(t, repo, "stash", "list"); out != "" {
		t.Fatalf("stash не удалён после y: %q", out)
	}
}
