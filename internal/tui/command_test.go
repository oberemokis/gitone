package tui_test

import (
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// commandRepo это репозиторий с правленым файлом «my file.txt», открытый с
// keybinds.json, где своя команда стоит на o.
func commandRepo(t *testing.T, command string) (string, tea.Model) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("команды в тесте написаны для sh")
	}

	repo := newRepo(t)
	write(t, repo, "my file.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "my file.txt", "two\n")

	keybinds := `{"commands":[{"key":"o","name":"mine","command":` + strings.ReplaceAll(`"`+command+`"`, `\`, `\\`) + `}]}`
	return repo, openWithKeybinds(t, repo, keybinds, 100)
}

// Команда без подстановки выполняется, первая строка вывода в подвале
// (#246, случай 1).
func TestCustomCommandOutput(t *testing.T) {
	_, model := commandRepo(t, "echo; echo first line; echo second")

	model = pressAndSettle(model, key('o'))
	if footer := footerOf(model); !strings.Contains(footer, "mine: first line") {
		t.Fatalf("подвал %q, ожидалась первая строка вывода", footer)
	}
}

// {path} подставлен в кавычках: пробел в имени команду не ломает (случай 2).
// Команда правит репозиторий, дифф перечитывается.
func TestCustomCommandPath(t *testing.T) {
	repo, model := commandRepo(t, "git add {path}")

	pressAndSettle(model, key('o'))
	if got := runGit(t, repo, "diff", "--cached", "--name-only"); got != "my file.txt" {
		t.Fatalf("в индексе %q, ожидался файл с пробелом", got)
	}
}

// Ошибка команды: код выхода и первая строка stderr (случай 3); команда,
// которая читает stdin, получает конец файла и не висит (случай 4).
func TestCustomCommandFailsAndDoesNotWait(t *testing.T) {
	_, model := commandRepo(t, "cat; echo broken >&2; exit 3")

	model = pressAndSettle(model, key('o'))
	if footer := footerOf(model); !strings.Contains(footer, "exit 3: broken") {
		t.Fatalf("подвал %q, ожидался код выхода и stderr", footer)
	}
}
