package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// answerYes отвечает y и выполняет то, что из этого вышло.
func answerYes(model tea.Model) tea.Model {
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	return settle(model, cmd, 4)
}

// Тихий опрос вопрос discard не трогает: тик приходит каждые две секунды, и
// вопрос иначе пропадал бы раньше, чем на него успеют ответить.
func TestTickKeepsDiscardQuestion(t *testing.T) {
	repo, model := editedRepo(t, oneTick(), layout.Focus())

	model, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	model = settle(model, model.Init(), 3)
	answerYes(model)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("a.txt = %q после y, want правка отменена", got)
	}
}

// Опрос, который принёс новый дифф, вопрос тоже не закрывает: тик без правок
// выходит раньше, чем дифф ставится на место, а этот доходит до самого конца.
func TestTickWithNewDiffKeepsDiscardQuestion(t *testing.T) {
	repo, model := editedRepo(t, oneTick(), layout.Focus())

	model, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	write(t, repo, "a.txt", "three\n")
	model = settle(model, model.Init(), 3)
	if body := screenText(model); !strings.Contains(body, "three") {
		t.Fatalf("тик не принёс новый дифф:\n%s", body)
	}
	answerYes(model)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("a.txt = %q после y, want правка отменена", got)
	}
}
