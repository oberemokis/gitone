package tui_test

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// commandRepo это репозиторий с правленым файлом «my file.txt», открытый с
// keybinds.json, где своя команда стоит на o.
func commandRepo(t *testing.T, command string) (string, tea.Model) {
	t.Helper()
	return commandRepoAt(t, "my file.txt", command)
}

// commandRepoAt это commandRepo с правленым файлом name.
func commandRepoAt(t *testing.T, name, command string) (string, tea.Model) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("команды в тесте написаны для sh")
	}

	repo := newRepo(t)
	write(t, repo, name, "one\n")
	commitAll(t, repo)
	write(t, repo, name, "two\n")

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

// Кавычка и $ в пути подставляются дословно: shell не разбирает путь
// повторно (#298, случай 1).
func TestCustomCommandPathIsLiteral(t *testing.T) {
	_, model := commandRepoAt(t, "it's $HOME.txt", "echo {path}")

	model = pressAndSettle(model, key('o'))
	if footer := footerOf(model); !strings.Contains(footer, "mine: it's $HOME.txt") {
		t.Fatalf("подвал %q, ожидался путь как есть", footer)
	}
}

// Фоновый процесс команды не держит итог (случай 2).
func TestCustomCommandDoesNotWaitForBackground(t *testing.T) {
	_, model := commandRepo(t, "sleep 5 & echo started")

	start := time.Now()
	model = pressAndSettle(model, key('o'))
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("команда шла %v, итог ждал фоновый процесс", elapsed)
	}
	if footer := footerOf(model); !strings.Contains(footer, "mine: started") {
		t.Fatalf("подвал %q, ожидался вывод до фона", footer)
	}
}

// stdin gitone команде не достаётся: go test сам даёт пустой stdin, поэтому
// тест подменяет его трубой с данными (случай 3).
func TestCustomCommandDoesNotReadOwnStdin(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString("secret\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	stdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() { os.Stdin = stdin; reader.Close() })

	_, model := commandRepo(t, "cat; echo end")
	model = pressAndSettle(model, key('o'))
	if footer := footerOf(model); !strings.Contains(footer, "mine: end") {
		t.Fatalf("подвал %q, команда прочитала stdin gitone", footer)
	}
}
