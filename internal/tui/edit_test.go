package tui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// editorRepo это репозиторий с правкой a.txt, открытый с запуском редактора без
// терминала: команда выполняется сразу, её итог идёт в обратный вызов, как
// после выхода из настоящего редактора.
func editorRepo(t *testing.T, visual string) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	t.Setenv("VISUAL", visual)
	t.Setenv("EDITOR", "")

	opts := options(repo, layout.Focus())
	opts.Exec = func(command *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		return func() tea.Msg { return done(command.Run()) }
	}
	return repo, resize(open(t, opts), 100, 20)
}

// e запускает редактор из VISUAL с путём файла, после выхода дифф перечитан и
// правка видна (#215, случаи 1 и 2). VISUAL с аргументами работает, как у git.
func TestEditOpensEditorAndRereads(t *testing.T) {
	repo, model := editorRepo(t, `printf 'edited\n' >>`)

	model = pressAndSettle(model, key('e'))

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "two\nedited\n" {
		t.Fatalf("a.txt = %q, редактор не получил путь", got)
	}
	if body := screenText(model); !strings.Contains(body, "edited") {
		t.Fatalf("дифф не перечитан после редактора:\n%s", body)
	}
}

// Без VISUAL и EDITOR подвал говорит, что задать, ничего не запускается
// (случай 3).
func TestEditWithoutEditor(t *testing.T) {
	_, model := editorRepo(t, "")

	model = pressAndSettle(model, key('e'))

	if footer := footerOf(model); !strings.Contains(footer, "set VISUAL or EDITOR") {
		t.Fatalf("подвал %q, ожидалась подсказка про редактор", footer)
	}
}

// Файл, удалённый после выбора, редактору не отдаётся: подвал показывает
// ошибку (случай 4).
func TestEditMissingFile(t *testing.T) {
	repo, model := editorRepo(t, "true")
	if err := os.Remove(filepath.Join(repo, "a.txt")); err != nil {
		t.Fatal(err)
	}

	model = pressAndSettle(model, key('e'))

	if footer := footerOf(model); !strings.Contains(footer, `"a.txt" no longer exists`) {
		t.Fatalf("подвал %q, ожидалась ошибка", footer)
	}
	if _, err := os.Stat(filepath.Join(repo, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("файл создан заново: %v", err)
	}
}
