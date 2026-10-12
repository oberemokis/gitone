package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/layout"
)

// Пробел ставит текущий файл в индекс. Отметки в панели нет (#99): индекс
// виден в git.
func TestSpaceStagesCurrentFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := toggleFiles(resize(open(t, options(repo, layout.Default())), 100, 20))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = settle(model, cmd, 4)

	if staged, err := git.StagedPaths(repo); err != nil || len(staged) != 1 || staged[0] != "a.txt" {
		t.Fatalf("в индексе %q, ошибка %v", staged, err)
	}
	if body := strings.Join(screenOf(model), "\n"); strings.Contains(body, "●") {
		t.Fatalf("в панели осталась отметка индекса:\n%s", body)
	}
}

// x спрашивает, и только y отменяет правку. Любая другая клавиша отменяет
// вопрос и не выполняет действие.
func TestDiscardNeedsYes(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if footer := footerOf(model); !strings.Contains(footer, "Discard changes") {
		t.Fatalf("вопроса нет в подвале: %q", footer)
	}

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	model = settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("правка пропала без подтверждения: %q", got)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("правка не отменена после y: %q", got)
	}
}
