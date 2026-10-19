package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/layout"
)

// x на строке, вопрос, над ней удалена строка файла, и на её номер встала
// соседняя: y отказывает, файл не изменён (#305, случай 1).
func TestDiscardSelectionAfterFileChanged(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "a\nb\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "a\nb\ndrop\nkeep\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 30)

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('v'))
	model, _ = model.Update(key('x'))
	write(t, repo, "a.txt", "b\ndrop\nkeep\n")
	model = answerYes(model)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "b\ndrop\nkeep\n" {
		t.Fatalf("a.txt = %q, отменена чужая строка", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "no longer outside the index") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}
