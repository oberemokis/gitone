package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// l открывает лог, enter показывает диф коммита, b возвращает рабочее дерево.
func TestLogShowsCommitDiffAndGoesBack(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	// в дереве правка, которой нет ни в одном коммите
	write(t, repo, "a.txt", "three\n")

	model := openStage(t, repo)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "commit") {
		t.Fatalf("лог не открылся:\n%s", body)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "one") || strings.Contains(body, "three") {
		t.Fatalf("дифф коммита не показан:\n%s", body)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "three") {
		t.Fatalf("рабочее дерево не вернулось:\n%s", body)
	}
}
