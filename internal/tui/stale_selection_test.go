package tui_test

import (
	"os"
	"path/filepath"
	"runtime"
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

// v, и до ответа git открыт коммит из лога: выделение не включается поверх
// диффа коммита (случай 2).
func TestSelectReplyAfterViewChanged(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "three\n")
	model := resize(open(t, options(repo, layout.Focus())), 140, 30)

	model, pending := model.Update(key('v'))
	model = openAsync(model, key('l'))
	model = enterKey(model)
	model, _ = model.Update(pending())

	if body := screenText(model); strings.Contains(body, "▌") {
		t.Fatalf("выделение включилось поверх диффа коммита:\n%s", body)
	}
}

// Правка только прав вне индекса хунков не даёт: подвал советует поставить
// файл целиком, а не говорит, что всё в индексе (случай 3).
func TestSelectModeChangeOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права файла в Windows git не видит")
	}
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	if err := os.Chmod(filepath.Join(repo, "a.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	model := resize(open(t, options(repo, layout.Focus())), 100, 30)

	model = pressAndSettle(model, key('v'))
	if footer := footerOf(model); !strings.Contains(footer, "stages this file whole") {
		t.Fatalf("подвал %q, ожидался совет поставить файл целиком", footer)
	}
}
