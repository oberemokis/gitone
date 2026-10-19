package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/layout"
)

// cloneWithSubmodule это клон репозитория с подмодулем libs/lib, сделанный
// без --recursive: подмодуль в нём не клонирован. Клон из локального пути
// git по умолчанию запрещает подмодулям, тест разрешает его через окружение.
func cloneWithSubmodule(t *testing.T) string {
	t.Helper()

	lib := newRepo(t)
	write(t, lib, "x.txt", "lib\n")
	commitAll(t, lib)

	repo := newRepo(t)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
	write(t, repo, "a.txt", "app\n")
	commitAll(t, repo)
	runGit(t, repo, "submodule", "add", "-q", lib, "libs/lib")
	runGit(t, repo, "commit", "-q", "-m", "submodule")

	clone := filepath.Join(t.TempDir(), "clone")
	runGit(t, filepath.Dir(clone), "clone", "-q", repo, clone)
	return clone
}

// S открывает окно: не клонированный подмодуль виден с пометкой, u клонирует
// его, подвал называет итог, а окно показывает новое состояние (#244,
// случаи 1–3).
func TestSubmodulesWindow(t *testing.T) {
	clone := cloneWithSubmodule(t)
	model := openAsync(resize(open(t, options(clone, layout.Focus())), 120, 20), key('S'))

	if row := markedRow(model); !strings.Contains(row, "libs/lib") || !strings.Contains(row, "not cloned") {
		t.Fatalf("строка подмодуля %q, ожидалась пометка", row)
	}

	model = pressAndSettle(model, key('u'))
	if got := readText(t, filepath.Join(clone, "libs", "lib", "x.txt")); got != "lib\n" {
		t.Fatalf("x.txt в подмодуле = %q", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Submodule libs/lib updated") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
	if row := markedRow(model); strings.Contains(row, "not cloned") {
		t.Fatalf("строка подмодуля %q после обновления", row)
	}
}

// Правка внутри подмодуля в панели файлов видна как подмодуль (случай 4).
func TestSubmoduleChangeLabel(t *testing.T) {
	clone := cloneWithSubmodule(t)
	runGit(t, clone, "submodule", "update", "-q", "--init")
	write(t, filepath.Join(clone, "libs", "lib"), "x.txt", "changed\n")

	model := toggleFiles(resize(open(t, options(clone, layout.Focus())), 120, 20))
	if body := screenText(model); !strings.Contains(body, "libs/lib submodule") {
		t.Fatalf("подмодуль не помечен в панели:\n%s", body)
	}
}
