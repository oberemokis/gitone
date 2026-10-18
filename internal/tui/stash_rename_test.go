package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// twoStashes это репозиторий с двумя записями stash «first» и «second»,
// открытый окном stash.
func twoStashes(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	for _, message := range []string{"first", "second"} {
		write(t, repo, "a.txt", message+"\n")
		runGit(t, repo, "stash", "push", "-q", "-m", message)
	}

	model := resize(open(t, options(repo, layout.Focus())), 120, 24)
	return repo, openAsync(model, key('Z'))
}

// r на нижней записи открывает поле с прежним сообщением, enter меняет его:
// запись остаётся на месте, курсор на ней, окно открыто (#237, случаи 1 и 2).
func TestStashRename(t *testing.T) {
	repo, model := twoStashes(t)
	model, _ = model.Update(key('j'))

	model, _ = model.Update(key('r'))
	if body := screenText(model); !strings.Contains(body, "Rename stash@{1}") || !strings.Contains(body, "> On main: first_") {
		t.Fatalf("r не открыл поле с прежним сообщением:\n%s", body)
	}
	model = enterKey(typeText(eraseText(model, len("On main: first")), "renamed"))

	if got := runGit(t, repo, "stash", "list", "--format=%gs"); got != "On main: second\nrenamed" {
		t.Fatalf("stash %q, ожидался прежний порядок с новым сообщением", got)
	}
	if row := markedRow(model); !strings.Contains(row, "stash@{1}  renamed") {
		t.Fatalf("курсор на %q, ожидалась переименованная запись", row)
	}
}

// Пустое сообщение отвергается с причиной в подвале (случай 3).
func TestStashRenameEmptyRefused(t *testing.T) {
	repo, model := twoStashes(t)

	model, _ = model.Update(key('r'))
	model = enterKey(eraseText(model, len("On main: second")))

	if footer := footerOf(model); !strings.Contains(footer, "the stash message cannot be empty") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
	if got := runGit(t, repo, "stash", "list", "--format=%gs"); got != "On main: second\nOn main: first" {
		t.Fatalf("stash %q изменился", got)
	}
}
